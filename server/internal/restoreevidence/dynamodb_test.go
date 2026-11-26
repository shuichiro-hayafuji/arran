package restoreevidence

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type fakeDynamo struct {
	get   func(*dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error)
	write func(*dynamodb.TransactWriteItemsInput) (*dynamodb.TransactWriteItemsOutput, error)
}

func (f fakeDynamo) GetItem(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	return f.get(in)
}
func (f fakeDynamo) TransactWriteItems(_ context.Context, in *dynamodb.TransactWriteItemsInput, _ ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	return f.write(in)
}
func TestDynamoStrongReadAndConditionalAtomicAppend(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	owner := "fedcba9876543210fedcba9876543210"
	calls := 0
	f := fakeDynamo{get: func(in *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
		if in.ConsistentRead == nil || !*in.ConsistentRead || *in.TableName != "fixture" {
			t.Fatal("not a consistent base table read")
		}
		return &dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{"pk": stringValue("HEAD#" + id), "generation": numberValue(0), "digest": stringValue("")}}, nil
	}, write: func(in *dynamodb.TransactWriteItemsInput) (*dynamodb.TransactWriteItemsOutput, error) {
		calls++
		if len(in.TransactItems) != 3 || in.TransactItems[0].Update == nil || in.TransactItems[1].Put == nil || in.TransactItems[0].Update.ConditionExpression == nil || in.TransactItems[1].Put.ConditionExpression == nil {
			t.Fatal("missing atomic compare and insert")
		}
		return &dynamodb.TransactWriteItemsOutput{}, nil
	}}
	s := DynamoStore{Client: f, Table: "fixture", LedgerID: id}
	head, err := s.ReadHead(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	e := Entry{LedgerID: id, OwnerID: owner, OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", PolicyVersion: "v1", Generation: 1, Kind: "withdrawal_requested"}
	e.Digest = Digest(e)
	if err = s.Append(context.Background(), head, e); err != nil || calls != 1 {
		t.Fatalf("append: %v calls %d", err, calls)
	}
	bad := e
	bad.Generation = 2
	if err = s.Append(context.Background(), head, bad); err == nil || calls != 1 {
		t.Fatal("invalid sequence was sent")
	}
}
func TestDynamoUnknownAndMissingFailClosed(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	owner := "fedcba9876543210fedcba9876543210"
	for _, out := range []*dynamodb.GetItemOutput{nil, {}, {Item: map[string]types.AttributeValue{"pk": stringValue("HEAD#" + id), "generation": numberValue(1), "digest": stringValue("")}}} {
		s := DynamoStore{Client: fakeDynamo{get: func(*dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) { return out, nil }}, Table: "fixture", LedgerID: id}
		if _, err := s.ReadHead(context.Background()); err == nil {
			t.Fatal("missing or invalid head accepted")
		}
	}
	e := Entry{LedgerID: id, OwnerID: owner, OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", PolicyVersion: "v1", Generation: 1, Kind: "send_started"}
	e.Digest = Digest(e)
	head := Head{LedgerID: id}
	for _, test := range []struct {
		out *dynamodb.TransactWriteItemsOutput
		err error
	}{{nil, nil}, {nil, errors.New("timeout")}, {nil, &types.TransactionCanceledException{}}} {
		s := DynamoStore{Client: fakeDynamo{write: func(*dynamodb.TransactWriteItemsInput) (*dynamodb.TransactWriteItemsOutput, error) {
			return test.out, test.err
		}}, Table: "fixture", LedgerID: id}
		if err := s.Append(context.Background(), head, e); !errors.Is(err, ErrUnknown) {
			t.Fatalf("uncertain write accepted: %v", err)
		}
	}
}

func TestDynamoEventReadFailsClosed(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	owner := "fedcba9876543210fedcba9876543210"
	e := Entry{LedgerID: id, OwnerID: owner, OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", PolicyVersion: "v1", Generation: 1, Kind: "withdrawal_requested"}
	e.Digest = Digest(e)
	item := map[string]types.AttributeValue{"pk": stringValue("EVENT#" + id + "#00000000000000000001"), "ledger_id": stringValue(id), "owner_id": stringValue(owner), "operation_id": stringValue(e.OperationID), "related_operation_id": stringValue(e.RelatedOperationID), "policy_version": stringValue(e.PolicyVersion), "kind": stringValue(e.Kind), "generation": numberValue(1), "previous": stringValue(""), "digest": stringValue(e.Digest)}
	calls := 0
	s := DynamoStore{Table: "fixture", LedgerID: id, Client: fakeDynamo{get: func(in *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
		calls++
		if in.ConsistentRead == nil || !*in.ConsistentRead {
			t.Fatal("event read not consistent")
		}
		if *in.TableName != "fixture" || in.Key["pk"].(*types.AttributeValueMemberS).Value != "EVENT#"+id+"#00000000000000000001" {
			t.Fatal("event read used wrong table or generation key")
		}
		return &dynamodb.GetItemOutput{Item: item}, nil
	}}}
	if got, err := s.ReadEvent(context.Background(), 1); err != nil || got != e {
		t.Fatalf("event: %+v %v", got, err)
	}
	item["owner_id"] = stringValue("00000000000000000000000000000000")
	if _, err := s.ReadEvent(context.Background(), 1); err == nil {
		t.Fatal("modified event accepted")
	}
	delete(item, "owner_id")
	if _, err := s.ReadEvent(context.Background(), 1); err == nil {
		t.Fatal("missing owner accepted")
	}
	if _, err := s.ReadEvent(context.Background(), 0); err == nil || calls != 3 {
		t.Fatal("zero generation sent")
	}
}

func TestDynamoConditionalCompareAndAmbiguousRetry(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	owner := "fedcba9876543210fedcba9876543210"
	generation := uint64(0)
	digest := ""
	entries := map[string]bool{}
	calls := 0
	f := fakeDynamo{write: func(in *dynamodb.TransactWriteItemsInput) (*dynamodb.TransactWriteItemsOutput, error) {
		calls++
		if len(in.TransactItems) != 3 {
			t.Fatal("transaction is not atomic")
		}
		u, p := in.TransactItems[0].Update, in.TransactItems[1].Put
		if u == nil || p == nil || *u.TableName != "fixture" || *p.TableName != "fixture" || u.Key["pk"].(*types.AttributeValueMemberS).Value != "HEAD#"+id || *u.ConditionExpression != "#g = :old AND #d = :previous" || *u.UpdateExpression != "SET #g = :next, #d = :digest" || u.ExpressionAttributeNames["#g"] != "generation" || u.ExpressionAttributeNames["#d"] != "digest" || *p.ConditionExpression != "attribute_not_exists(pk)" {
			t.Fatal("invalid conditional transaction")
		}
		value := u.ExpressionAttributeValues
		old := value[":old"].(*types.AttributeValueMemberN).Value
		previous := value[":previous"].(*types.AttributeValueMemberS).Value
		next := value[":next"].(*types.AttributeValueMemberN).Value
		newDigest := value[":digest"].(*types.AttributeValueMemberS).Value
		key := p.Item["pk"].(*types.AttributeValueMemberS).Value
		if key != "EVENT#"+id+"#00000000000000000001" || p.Item["ledger_id"].(*types.AttributeValueMemberS).Value != id || p.Item["owner_id"].(*types.AttributeValueMemberS).Value != owner || p.Item["generation"].(*types.AttributeValueMemberN).Value != next || p.Item["previous"].(*types.AttributeValueMemberS).Value != previous || p.Item["digest"].(*types.AttributeValueMemberS).Value != newDigest {
			t.Fatal("wrong event identity or state")
		}
		if old != strconv.FormatUint(generation, 10) || previous != digest || entries[key] {
			return nil, &types.TransactionCanceledException{}
		}
		if next != "1" {
			t.Fatal("wrong next generation")
		}
		generation = 1
		digest = newDigest
		entries[key] = true
		return nil, errors.New("response lost after commit")
	}}
	s := DynamoStore{Client: f, Table: "fixture", LedgerID: id}
	head := Head{LedgerID: id}
	e := Entry{LedgerID: id, OwnerID: owner, OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", PolicyVersion: "v1", Generation: 1, Kind: "withdrawal_requested"}
	e.Digest = Digest(e)
	if err := s.Append(context.Background(), head, e); !errors.Is(err, ErrUnknown) {
		t.Fatal("lost response accepted")
	}
	if err := s.Append(context.Background(), head, e); !errors.Is(err, ErrUnknown) {
		t.Fatal("same retry accepted")
	}
	alternate := e
	alternate.Kind = "consent_withdrawn"
	alternate.Digest = Digest(alternate)
	if err := s.Append(context.Background(), head, alternate); !errors.Is(err, ErrUnknown) {
		t.Fatal("competing append accepted")
	}
	if calls != 3 || generation != 1 || digest != e.Digest || len(entries) != 1 {
		t.Fatal("competing operations changed committed event")
	}
}

func TestDynamoSecondGenerationUsesPreviousDigestAndEventKind(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	owner := "fedcba9876543210fedcba9876543210"
	first := Entry{LedgerID: id, OwnerID: owner, OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", PolicyVersion: "v1", Generation: 1, Kind: "consent_granted"}
	first.Digest = Digest(first)
	second := Entry{LedgerID: id, OwnerID: owner, OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", PolicyVersion: "v1", Generation: 2, Kind: "consent_withdrawn", Previous: first.Digest}
	second.Digest = Digest(second)
	calls := 0
	s := DynamoStore{LedgerID: id, Table: "fixture", Client: fakeDynamo{write: func(in *dynamodb.TransactWriteItemsInput) (*dynamodb.TransactWriteItemsOutput, error) {
		calls++
		u, p := in.TransactItems[0].Update, in.TransactItems[1].Put
		v := u.ExpressionAttributeValues
		if v[":old"].(*types.AttributeValueMemberN).Value != "1" || v[":previous"].(*types.AttributeValueMemberS).Value != first.Digest || v[":next"].(*types.AttributeValueMemberN).Value != "2" || v[":digest"].(*types.AttributeValueMemberS).Value != second.Digest || p.Item["kind"].(*types.AttributeValueMemberS).Value != second.Kind || p.Item["pk"].(*types.AttributeValueMemberS).Value != "EVENT#"+id+"#00000000000000000002" {
			t.Fatal("second generation did not use previous state and event kind")
		}
		return &dynamodb.TransactWriteItemsOutput{}, nil
	}}}
	if err := s.Append(context.Background(), Head{LedgerID: id, Generation: 1, Digest: first.Digest}, second); err != nil || calls != 1 {
		t.Fatalf("second generation: %v calls=%d", err, calls)
	}
}

func TestDynamoOperationIDCannotBeReusedAtNewHead(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	owner := "fedcba9876543210fedcba9876543210"
	op := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	operations := map[string]bool{}
	fake := fakeDynamo{write: func(in *dynamodb.TransactWriteItemsInput) (*dynamodb.TransactWriteItemsOutput, error) {
		if len(in.TransactItems) != 3 || in.TransactItems[2].Put == nil {
			t.Fatal("operation uniqueness missing")
		}
		p := in.TransactItems[2].Put
		key := p.Item["pk"].(*types.AttributeValueMemberS).Value
		if *p.TableName != "fixture" || *p.ConditionExpression != "attribute_not_exists(pk)" || key != "OP#"+id+"#"+op || p.Item["owner_id"].(*types.AttributeValueMemberS).Value != owner {
			t.Fatal("wrong operation identity boundary")
		}
		if operations[key] {
			return nil, &types.TransactionCanceledException{}
		}
		operations[key] = true
		return &dynamodb.TransactWriteItemsOutput{}, nil
	}}
	s := DynamoStore{Table: "fixture", LedgerID: id, Client: fake}
	first := Entry{LedgerID: id, OwnerID: owner, OperationID: op, Generation: 1, Kind: "withdrawal_requested"}
	first.Digest = Digest(first)
	if err := s.Append(context.Background(), Head{LedgerID: id}, first); err != nil {
		t.Fatal(err)
	}
	second := Entry{LedgerID: id, OwnerID: owner, OperationID: op, Generation: 2, Previous: first.Digest, Kind: "withdrawal_completed"}
	second.Digest = Digest(second)
	if err := s.Append(context.Background(), Head{LedgerID: id, Generation: 1, Digest: first.Digest}, second); !errors.Is(err, ErrUnknown) {
		t.Fatal("reused operation was accepted")
	}
}

func TestConfirmOperationChecksOwnerAndEvent(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	owner := "fedcba9876543210fedcba9876543210"
	op := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	e := Entry{LedgerID: id, OwnerID: owner, OperationID: op, Generation: 1, Kind: "withdrawal_requested"}
	e.Digest = Digest(e)
	receipt := map[string]types.AttributeValue{"pk": stringValue("OP#" + id + "#" + op), "owner_id": stringValue(owner), "kind": stringValue(e.Kind), "policy_version": stringValue(e.PolicyVersion), "generation": numberValue(1), "digest": stringValue(e.Digest)}
	event := map[string]types.AttributeValue{"pk": stringValue("EVENT#" + id + "#00000000000000000001"), "ledger_id": stringValue(id), "owner_id": stringValue(owner), "operation_id": stringValue(op), "related_operation_id": stringValue(""), "policy_version": stringValue(""), "kind": stringValue(e.Kind), "generation": numberValue(1), "previous": stringValue(""), "digest": stringValue(e.Digest)}
	f := fakeDynamo{get: func(in *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
		if in.ConsistentRead == nil || !*in.ConsistentRead || *in.TableName != "fixture" {
			t.Fatal("untrusted lookup")
		}
		switch in.Key["pk"].(*types.AttributeValueMemberS).Value {
		case "OP#" + id + "#" + op:
			return &dynamodb.GetItemOutput{Item: receipt}, nil
		case "EVENT#" + id + "#00000000000000000001":
			return &dynamodb.GetItemOutput{Item: event}, nil
		default:
			t.Fatal("wrong key")
			return nil, nil
		}
	}}
	s := DynamoStore{Client: f, Table: "fixture", LedgerID: id}
	if got, err := s.ConfirmOperation(context.Background(), e); err != nil || got != e {
		t.Fatalf("confirmation: %+v %v", got, err)
	}
	delete(receipt, "kind")
	if _, err := s.ConfirmOperation(context.Background(), e); err == nil {
		t.Fatal("missing receipt kind confirmed")
	}
	receipt["kind"] = stringValue("send_started")
	if _, err := s.ConfirmOperation(context.Background(), e); err == nil {
		t.Fatal("wrong receipt kind confirmed")
	}
	receipt["kind"] = stringValue(e.Kind)
	wrongKind := e
	wrongKind.Kind = "withdrawal_completed"
	if _, err := s.ConfirmOperation(context.Background(), wrongKind); err == nil {
		t.Fatal("different kind confirmed")
	}
	wrongPolicy := e
	wrongPolicy.PolicyVersion = "different"
	if _, err := s.ConfirmOperation(context.Background(), wrongPolicy); err == nil {
		t.Fatal("different policy confirmed")
	}
	other := e
	other.OwnerID = "11111111111111111111111111111111"
	if _, err := s.ConfirmOperation(context.Background(), other); err == nil {
		t.Fatal("another owner confirmed operation")
	}
	event["kind"] = stringValue("withdrawal_completed")
	if _, err := s.ConfirmOperation(context.Background(), e); err == nil {
		t.Fatal("modified event confirmed")
	}
}

func TestIdentifierCaseVariantsRejected(t *testing.T) {
	id := "abcdefabcdefabcdefabcdefabcdefab"
	owner := "1234567890abcdef1234567890abcdef"
	op := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	e := Entry{LedgerID: id, OwnerID: owner, OperationID: op, Generation: 1, Kind: "withdrawal_requested"}
	e.Digest = Digest(e)
	h := Head{LedgerID: id, Generation: 1, Digest: e.Digest}
	if err := ValidateChain(h, []Entry{e}); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Entry){func(x *Entry) { x.LedgerID = strings.ToUpper(x.LedgerID) }, func(x *Entry) { x.OwnerID = strings.ToUpper(x.OwnerID) }, func(x *Entry) { x.OperationID = strings.ToUpper(x.OperationID) }} {
		x := e
		mutate(&x)
		x.Digest = Digest(x)
		bad := h
		bad.Digest = x.Digest
		if ValidateChain(bad, []Entry{x}) == nil {
			t.Fatal("case variant accepted")
		}
	}
	s := DynamoStore{Table: "fixture", LedgerID: strings.ToUpper(id), Client: fakeDynamo{}}
	if _, err := s.ReadHead(context.Background()); err == nil {
		t.Fatal("case variant lookup accepted")
	}
	s.LedgerID = id
	e.OperationID = strings.ToUpper(op)
	e.Digest = Digest(e)
	if err := s.Append(context.Background(), Head{LedgerID: id}, e); err == nil {
		t.Fatal("case variant append accepted")
	}
}

func TestDynamoSendCompletionChecksStartOwnerAndUniqueFinish(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	owner := "fedcba9876543210fedcba9876543210"
	start := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	finish := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	prev := Entry{LedgerID: id, OwnerID: owner, OperationID: start, PolicyVersion: "v1", Generation: 1, Kind: "send_started"}
	prev.Digest = Digest(prev)
	e := Entry{LedgerID: id, OwnerID: owner, OperationID: finish, RelatedOperationID: start, PolicyVersion: "v1", Generation: 2, Kind: "send_completed", Previous: prev.Digest}
	e.Digest = Digest(e)
	head := Head{LedgerID: id, Generation: 2, Digest: e.Digest}
	if err := ValidateChain(head, []Entry{prev, e}); err != nil {
		t.Fatal(err)
	}
	wrong := e
	wrong.OwnerID = "11111111111111111111111111111111"
	wrong.Digest = Digest(wrong)
	h := head
	h.Digest = wrong.Digest
	if ValidateChain(h, []Entry{prev, wrong}) == nil {
		t.Fatal("other owner completed start")
	}
	f := fakeDynamo{write: func(in *dynamodb.TransactWriteItemsInput) (*dynamodb.TransactWriteItemsOutput, error) {
		if len(in.TransactItems) != 5 {
			t.Fatal("start and finish conditions missing")
		}
		check, put := in.TransactItems[3].ConditionCheck, in.TransactItems[4].Put
		if check == nil || put == nil || *check.TableName != "fixture" || *put.TableName != "fixture" || check.Key["pk"].(*types.AttributeValueMemberS).Value != "OP#"+id+"#"+start || *check.ConditionExpression != "owner_id = :owner AND #kind = :started AND policy_version = :policy" || check.ExpressionAttributeValues[":owner"].(*types.AttributeValueMemberS).Value != owner || check.ExpressionAttributeValues[":started"].(*types.AttributeValueMemberS).Value != "send_started" || *put.ConditionExpression != "attribute_not_exists(pk)" || put.Item["pk"].(*types.AttributeValueMemberS).Value != "FINISH#"+id+"#"+start {
			t.Fatal("invalid start/finish conditions")
		}
		return &dynamodb.TransactWriteItemsOutput{}, nil
	}}
	s := DynamoStore{Client: f, Table: "fixture", LedgerID: id}
	if err := s.Append(context.Background(), Head{LedgerID: id, Generation: 1, Digest: prev.Digest}, e); err != nil {
		t.Fatal(err)
	}
}

func TestSendLifecycleWithLostReplyAndCompetingCompletion(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	owner := "fedcba9876543210fedcba9876543210"
	start := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	finish := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	rows := map[string]map[string]types.AttributeValue{}
	generation := uint64(0)
	digest := ""
	lost := false
	f := fakeDynamo{}
	f.get = func(in *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
		if in.ConsistentRead == nil || !*in.ConsistentRead || *in.TableName != "fixture" {
			t.Fatal("lookup not consistent")
		}
		key := in.Key["pk"].(*types.AttributeValueMemberS).Value
		return &dynamodb.GetItemOutput{Item: rows[key]}, nil
	}
	f.write = func(in *dynamodb.TransactWriteItemsInput) (*dynamodb.TransactWriteItemsOutput, error) {
		if len(in.TransactItems) < 3 {
			t.Fatal("missing items")
		}
		u := in.TransactItems[0].Update
		vals := u.ExpressionAttributeValues
		if vals[":old"].(*types.AttributeValueMemberN).Value != strconv.FormatUint(generation, 10) || vals[":previous"].(*types.AttributeValueMemberS).Value != digest {
			return nil, &types.TransactionCanceledException{}
		}
		for _, it := range in.TransactItems[1:] {
			if it.Put != nil {
				key := it.Put.Item["pk"].(*types.AttributeValueMemberS).Value
				if rows[key] != nil {
					return nil, &types.TransactionCanceledException{}
				}
			}
			if it.ConditionCheck != nil {
				k := it.ConditionCheck.Key["pk"].(*types.AttributeValueMemberS).Value
				row := rows[k]
				if row == nil || row["owner_id"].(*types.AttributeValueMemberS).Value != it.ConditionCheck.ExpressionAttributeValues[":owner"].(*types.AttributeValueMemberS).Value || row["kind"].(*types.AttributeValueMemberS).Value != "send_started" || row["policy_version"].(*types.AttributeValueMemberS).Value != it.ConditionCheck.ExpressionAttributeValues[":policy"].(*types.AttributeValueMemberS).Value {
					return nil, &types.TransactionCanceledException{}
				}
			}
		}
		for _, it := range in.TransactItems[1:] {
			if it.Put != nil {
				key := it.Put.Item["pk"].(*types.AttributeValueMemberS).Value
				rows[key] = it.Put.Item
			}
		}
		generation++
		digest = vals[":digest"].(*types.AttributeValueMemberS).Value
		if generation == 2 && !lost {
			lost = true
			return nil, errors.New("response lost")
		}
		return &dynamodb.TransactWriteItemsOutput{}, nil
	}
	s := DynamoStore{Client: f, Table: "fixture", LedgerID: id}
	first := Entry{LedgerID: id, OwnerID: owner, OperationID: start, PolicyVersion: "v1", Generation: 1, Kind: "send_started"}
	first.Digest = Digest(first)
	if err := s.Append(context.Background(), Head{LedgerID: id}, first); err != nil {
		t.Fatal(err)
	}
	done := Entry{LedgerID: id, OwnerID: owner, OperationID: finish, RelatedOperationID: start, PolicyVersion: "v1", Generation: 2, Kind: "send_completed", Previous: first.Digest}
	done.Digest = Digest(done)
	wrongPolicy := done
	wrongPolicy.PolicyVersion = "v2"
	wrongPolicy.Digest = Digest(wrongPolicy)
	if err := s.Append(context.Background(), Head{LedgerID: id, Generation: 1, Digest: first.Digest}, wrongPolicy); !errors.Is(err, ErrUnknown) {
		t.Fatal("different start policy accepted")
	}
	if generation != 1 || digest != first.Digest {
		t.Fatal("head changed on policy mismatch")
	}
	if ValidateChain(Head{LedgerID: id, Generation: 2, Digest: wrongPolicy.Digest}, []Entry{first, wrongPolicy}) == nil {
		t.Fatal("different policy chain accepted")
	}
	if err := s.Append(context.Background(), Head{LedgerID: id, Generation: 1, Digest: first.Digest}, done); !errors.Is(err, ErrUnknown) {
		t.Fatal("lost response confirmed")
	}
	if got, err := s.ConfirmOperation(context.Background(), done); err != nil || got != done {
		t.Fatalf("readback: %+v %v", got, err)
	}
	duplicate := done
	duplicate.OperationID = "cccccccccccccccccccccccccccccccc"
	duplicate.Generation = 3
	duplicate.Previous = done.Digest
	duplicate.Digest = Digest(duplicate)
	if err := s.Append(context.Background(), Head{LedgerID: id, Generation: 2, Digest: done.Digest}, duplicate); !errors.Is(err, ErrUnknown) {
		t.Fatal("duplicate completion accepted")
	}
	if generation != 2 || digest != done.Digest {
		t.Fatal("head advanced on rejected completion")
	}
	if ValidateChain(Head{LedgerID: id, Generation: 3, Digest: duplicate.Digest}, []Entry{first, done, duplicate}) == nil {
		t.Fatal("duplicate completion chain accepted")
	}
}

func TestLookupAbsenceIsNotAnErrorOrNilResponse(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	e := Entry{LedgerID: id, OwnerID: "fedcba9876543210fedcba9876543210", OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: "withdrawal_requested"}
	var out *dynamodb.GetItemOutput
	var failure error
	fake := fakeDynamo{get: func(in *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
		if in.ConsistentRead == nil || !*in.ConsistentRead || *in.TableName != "fixture" || in.Key["pk"].(*types.AttributeValueMemberS).Value != "OP#"+id+"#"+e.OperationID {
			t.Fatal("lookup was not an exact strong read")
		}
		return out, failure
	}}
	s := DynamoStore{Client: fake, Table: "fixture", LedgerID: id}
	if state, _, err := s.LookupOperation(context.Background(), e); state != LookupUnknown || err == nil {
		t.Fatal("nil response treated as absence")
	}
	out = &dynamodb.GetItemOutput{}
	failure = errors.New("timeout")
	if state, _, err := s.LookupOperation(context.Background(), e); state != LookupUnknown || err == nil {
		t.Fatal("failed read treated as absence")
	}
	failure = nil
	if state, _, err := s.LookupOperation(context.Background(), e); state != LookupAbsent || err != nil {
		t.Fatal("confirmed missing operation not absent")
	}
}

func TestLookupRejectsFirstContradictoryReceipt(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	e := Entry{LedgerID: id, OwnerID: "fedcba9876543210fedcba9876543210", OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: "withdrawal_requested"}
	calls := 0
	f := fakeDynamo{get: func(*dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
		calls++
		return &dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{"pk": stringValue("OP#" + id + "#" + e.OperationID), "owner_id": stringValue("11111111111111111111111111111111")}}, nil
	}}
	s := DynamoStore{Client: f, Table: "fixture", LedgerID: id}
	state, _, err := s.LookupOperation(context.Background(), e)
	if state != LookupUnknown || err == nil || calls != 1 {
		t.Fatal("contradictory first receipt ignored")
	}
}
