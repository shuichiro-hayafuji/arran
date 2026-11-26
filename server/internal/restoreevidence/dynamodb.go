package restoreevidence

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoAPI is implemented by the official DynamoDB SDK client. A production
// caller must separately establish account, region, table identity and IAM.
type DynamoAPI interface {
	GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	TransactWriteItems(context.Context, *dynamodb.TransactWriteItemsInput, ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
}
type DynamoStore struct {
	Client   DynamoAPI
	Table    string
	LedgerID string
}

var ErrUnknown = errors.New("evidence operation not confirmed")

func (s DynamoStore) valid() bool               { return s.Client != nil && s.Table != "" && validID(s.LedgerID) }
func stringValue(v string) types.AttributeValue { return &types.AttributeValueMemberS{Value: v} }
func numberValue(v uint64) types.AttributeValue {
	return &types.AttributeValueMemberN{Value: strconv.FormatUint(v, 10)}
}
func pk(v string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{"pk": stringValue(v)}
}
func (s DynamoStore) headKey() string { return "HEAD#" + s.LedgerID }

// ReadHead uses a base-table strongly consistent read. Missing, malformed,
// unavailable and wrong-ledger results are all failures. This method does not
// itself prove that the configured account/table is the intended authority.
func (s DynamoStore) ReadHead(ctx context.Context) (Head, error) {
	if !s.valid() {
		return Head{}, ErrUnknown
	}
	out, err := s.Client.GetItem(ctx, &dynamodb.GetItemInput{TableName: &s.Table, Key: pk(s.headKey()), ConsistentRead: boolptr(true)})
	if err != nil || out == nil || out.Item == nil {
		return Head{}, ErrUnknown
	}
	key, k := out.Item["pk"].(*types.AttributeValueMemberS)
	generation, g := out.Item["generation"].(*types.AttributeValueMemberN)
	digest, d := out.Item["digest"].(*types.AttributeValueMemberS)
	if !k || !g || !d || key.Value != s.headKey() {
		return Head{}, ErrUnknown
	}
	n, err := strconv.ParseUint(generation.Value, 10, 64)
	if err != nil || (n == 0 && digest.Value != "") || (n > 0 && !validDigest(digest.Value)) {
		return Head{}, ErrUnknown
	}
	return Head{LedgerID: s.LedgerID, Generation: n, Digest: digest.Value}, nil
}
func boolptr(v bool) *bool { return &v }

// Append atomically creates one immutable event and advances the expected head.
// Every failed or uncertain response is unconfirmed; callers must not proceed
// with an external side effect or infer that retrying is safe from this error.
func (s DynamoStore) Append(ctx context.Context, expected Head, e Entry) error {
	if !s.valid() || expected.LedgerID != s.LedgerID || e.LedgerID != s.LedgerID || !validID(e.OwnerID) || !validID(e.OperationID) || expected.Generation == ^uint64(0) || e.Generation != expected.Generation+1 || e.Previous != expected.Digest || e.Digest != Digest(e) {
		return ErrUnknown
	}
	if (expected.Generation == 0 && expected.Digest != "") || (expected.Generation > 0 && !validDigest(expected.Digest)) {
		return ErrUnknown
	}
	switch e.Kind {
	case "consent_granted", "consent_withdrawn":
		if e.PolicyVersion == "" {
			return ErrUnknown
		}
	case "send_completed":
		if !validID(e.RelatedOperationID) || e.RelatedOperationID == e.OperationID || e.PolicyVersion == "" {
			return ErrUnknown
		}
	case "send_started":
		if e.PolicyVersion == "" {
			return ErrUnknown
		}
	case "withdrawal_requested", "withdrawal_completed":
	default:
		return ErrUnknown
	}
	if e.Kind != "send_completed" && e.RelatedOperationID != "" {
		return ErrUnknown
	}
	eventKey := fmt.Sprintf("EVENT#%s#%020d", s.LedgerID, e.Generation)
	conditionHead := "#g = :old AND #d = :previous"
	conditionEvent := "attribute_not_exists(pk)"
	update := "SET #g = :next, #d = :digest"
	items := []types.TransactWriteItem{
		{Update: &types.Update{TableName: &s.Table, Key: pk(s.headKey()), ConditionExpression: &conditionHead, UpdateExpression: &update, ExpressionAttributeNames: map[string]string{"#g": "generation", "#d": "digest"}, ExpressionAttributeValues: map[string]types.AttributeValue{":old": numberValue(expected.Generation), ":previous": stringValue(expected.Digest), ":next": numberValue(e.Generation), ":digest": stringValue(e.Digest)}}},
		{Put: &types.Put{TableName: &s.Table, ConditionExpression: &conditionEvent, Item: map[string]types.AttributeValue{"pk": stringValue(eventKey), "ledger_id": stringValue(e.LedgerID), "owner_id": stringValue(e.OwnerID), "operation_id": stringValue(e.OperationID), "related_operation_id": stringValue(e.RelatedOperationID), "policy_version": stringValue(e.PolicyVersion), "kind": stringValue(e.Kind), "generation": numberValue(e.Generation), "previous": stringValue(e.Previous), "digest": stringValue(e.Digest)}}},
		{Put: &types.Put{TableName: &s.Table, ConditionExpression: &conditionEvent, Item: map[string]types.AttributeValue{"pk": stringValue("OP#" + s.LedgerID + "#" + e.OperationID), "owner_id": stringValue(e.OwnerID), "policy_version": stringValue(e.PolicyVersion), "kind": stringValue(e.Kind), "generation": numberValue(e.Generation), "digest": stringValue(e.Digest)}}},
	}
	if e.Kind == "send_completed" {
		conditionStart := "owner_id = :owner AND #kind = :started AND policy_version = :policy"
		items = append(items,
			types.TransactWriteItem{ConditionCheck: &types.ConditionCheck{TableName: &s.Table, Key: pk("OP#" + s.LedgerID + "#" + e.RelatedOperationID), ConditionExpression: &conditionStart, ExpressionAttributeNames: map[string]string{"#kind": "kind"}, ExpressionAttributeValues: map[string]types.AttributeValue{":owner": stringValue(e.OwnerID), ":started": stringValue("send_started"), ":policy": stringValue(e.PolicyVersion)}}},
			types.TransactWriteItem{Put: &types.Put{TableName: &s.Table, ConditionExpression: &conditionEvent, Item: map[string]types.AttributeValue{"pk": stringValue("FINISH#" + s.LedgerID + "#" + e.RelatedOperationID), "owner_id": stringValue(e.OwnerID), "operation_id": stringValue(e.OperationID)}}})
	}
	out, err := s.Client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: items})
	if err != nil || out == nil {
		return ErrUnknown
	}
	return nil
}

// ReadEvent reads by exact generation from the base table. The caller must
// validate the full ordered chain against a trusted head while writers are
// fenced; a missing item is never treated as an empty history.
func (s DynamoStore) ReadEvent(ctx context.Context, generation uint64) (Entry, error) {
	if !s.valid() || generation == 0 {
		return Entry{}, ErrUnknown
	}
	eventKey := fmt.Sprintf("EVENT#%s#%020d", s.LedgerID, generation)
	out, err := s.Client.GetItem(ctx, &dynamodb.GetItemInput{TableName: &s.Table, Key: pk(eventKey), ConsistentRead: boolptr(true)})
	if err != nil || out == nil || out.Item == nil {
		return Entry{}, ErrUnknown
	}
	str := func(name string) (string, bool) {
		v, ok := out.Item[name].(*types.AttributeValueMemberS)
		if !ok {
			return "", false
		}
		return v.Value, true
	}
	key, k := str("pk")
	ledger, l := str("ledger_id")
	owner, o := str("owner_id")
	operation, op := str("operation_id")
	related, rel := str("related_operation_id")
	policy, pv := str("policy_version")
	kind, c := str("kind")
	previous, p := str("previous")
	digest, d := str("digest")
	number, n := out.Item["generation"].(*types.AttributeValueMemberN)
	if !k || !l || !o || !op || !rel || !pv || !c || !p || !d || !n || key != eventKey || ledger != s.LedgerID {
		return Entry{}, ErrUnknown
	}
	gen, err := strconv.ParseUint(number.Value, 10, 64)
	if err != nil || gen != generation || !validID(owner) || !validID(operation) {
		return Entry{}, ErrUnknown
	}
	e := Entry{LedgerID: ledger, Generation: gen, OwnerID: owner, OperationID: operation, RelatedOperationID: related, PolicyVersion: policy, Kind: kind, Previous: previous, Digest: digest}
	switch e.Kind {
	case "consent_granted", "consent_withdrawn":
		if e.PolicyVersion == "" {
			return Entry{}, ErrUnknown
		}
	case "send_completed":
		if !validID(e.RelatedOperationID) || e.RelatedOperationID == e.OperationID || e.PolicyVersion == "" {
			return Entry{}, ErrUnknown
		}
	case "send_started":
		if e.PolicyVersion == "" {
			return Entry{}, ErrUnknown
		}
	case "withdrawal_requested", "withdrawal_completed":
	default:
		return Entry{}, ErrUnknown
	}
	if e.Kind != "send_completed" && e.RelatedOperationID != "" {
		return Entry{}, ErrUnknown
	}
	if !validDigest(e.Digest) || Digest(e) != e.Digest {
		return Entry{}, ErrUnknown
	}
	return e, nil
}

// ConfirmOperation reads the unique operation receipt, then verifies the event
// from the same ledger. A mismatch, absence, or read failure remains unknown.
// This alone does not authorize replay or prove that writes are frozen.
func (s DynamoStore) ConfirmOperation(ctx context.Context, expected Entry) (Entry, error) {
	operationID, ownerID := expected.OperationID, expected.OwnerID
	if !s.valid() || expected.LedgerID != s.LedgerID || !validID(operationID) || !validID(ownerID) || expected.Kind == "" {
		return Entry{}, ErrUnknown
	}
	key := "OP#" + s.LedgerID + "#" + operationID
	out, err := s.Client.GetItem(ctx, &dynamodb.GetItemInput{TableName: &s.Table, Key: pk(key), ConsistentRead: boolptr(true)})
	if err != nil || out == nil || out.Item == nil {
		return Entry{}, ErrUnknown
	}
	return s.confirmOperationItem(ctx, expected, out.Item)
}
func (s DynamoStore) confirmOperationItem(ctx context.Context, expected Entry, item map[string]types.AttributeValue) (Entry, error) {
	operationID, ownerID := expected.OperationID, expected.OwnerID
	key := "OP#" + s.LedgerID + "#" + operationID
	k, ok := item["pk"].(*types.AttributeValueMemberS)
	owner, okOwner := item["owner_id"].(*types.AttributeValueMemberS)
	kind, okKind := item["kind"].(*types.AttributeValueMemberS)
	policy, okPolicy := item["policy_version"].(*types.AttributeValueMemberS)
	num, okNum := item["generation"].(*types.AttributeValueMemberN)
	digest, okDigest := item["digest"].(*types.AttributeValueMemberS)
	if !ok || !okOwner || !okKind || !okPolicy || !okNum || !okDigest || k.Value != key || owner.Value != ownerID || kind.Value != expected.Kind || policy.Value != expected.PolicyVersion || !validDigest(digest.Value) {
		return Entry{}, ErrUnknown
	}
	gen, err := strconv.ParseUint(num.Value, 10, 64)
	if err != nil || gen == 0 {
		return Entry{}, ErrUnknown
	}
	event, err := s.ReadEvent(ctx, gen)
	if err != nil || event.OperationID != operationID || event.OwnerID != ownerID || event.Kind != expected.Kind || event.RelatedOperationID != expected.RelatedOperationID || event.PolicyVersion != expected.PolicyVersion || event.Digest != digest.Value {
		return Entry{}, ErrUnknown
	}
	return event, nil
}

// LookupOperation distinguishes a confirmed empty operation key from errors.
// Absence is meaningful only when the caller has independently authenticated
// the table and fenced every writer. This package does not establish either.
type LookupResult int

const (
	LookupUnknown LookupResult = iota
	LookupAbsent
	LookupFound
)

func (s DynamoStore) LookupOperation(ctx context.Context, expected Entry) (LookupResult, Entry, error) {
	if !s.valid() || expected.LedgerID != s.LedgerID || !validID(expected.OperationID) || !validID(expected.OwnerID) || expected.Kind == "" {
		return LookupUnknown, Entry{}, ErrUnknown
	}
	out, err := s.Client.GetItem(ctx, &dynamodb.GetItemInput{TableName: &s.Table, Key: pk("OP#" + s.LedgerID + "#" + expected.OperationID), ConsistentRead: boolptr(true)})
	if err != nil || out == nil {
		return LookupUnknown, Entry{}, ErrUnknown
	}
	if len(out.Item) == 0 {
		return LookupAbsent, Entry{}, nil
	}
	event, err := s.confirmOperationItem(ctx, expected, out.Item)
	if err != nil {
		return LookupUnknown, Entry{}, ErrUnknown
	}
	return LookupFound, event, nil
}
