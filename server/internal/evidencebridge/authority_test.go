package evidencebridge

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/shuichirohayafuji/spendable-today/server/internal/evidenceauthority"
	"github.com/shuichirohayafuji/spendable-today/server/internal/evidenceflow"
	"github.com/shuichirohayafuji/spendable-today/server/internal/restoreevidence"
	"testing"
)

type stubIdentity struct{}

func (stubIdentity) GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	a := "123456789012"
	arn := "arn:aws:iam::123456789012:user/fixture"
	return &sts.GetCallerIdentityOutput{Account: &a, Arn: &arn}, nil
}

type stubDB struct {
	token string
	rows  map[string]map[string]types.AttributeValue
	fail  bool
}

func (*stubDB) DescribeTable(context.Context, *dynamodb.DescribeTableInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	arn := "arn:aws:dynamodb:ap-northeast-1:123456789012:table/evidence"
	id := "fixture-id"
	return &dynamodb.DescribeTableOutput{Table: &types.TableDescription{TableArn: &arn, TableId: &id, TableStatus: types.TableStatusActive}}, nil
}
func (d *stubDB) GetItem(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if d.fail {
		return nil, errors.New("read failed")
	}
	pk := in.Key["pk"].(*types.AttributeValueMemberS).Value
	if pk == "LEASE#0123456789abcdef0123456789abcdef" {
		return &dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{"token": &types.AttributeValueMemberS{Value: d.token}}}, nil
	}
	return &dynamodb.GetItemOutput{Item: d.rows[pk]}, nil
}
func (d *stubDB) TransactWriteItems(_ context.Context, in *dynamodb.TransactWriteItemsInput, _ ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	d.token = in.TransactItems[0].Put.Item["token"].(*types.AttributeValueMemberS).Value
	return &dynamodb.TransactWriteItemsOutput{}, nil
}
func TestBridgeBindsTableLedgerAndSession(t *testing.T) {
	ctx := context.Background()
	id := "0123456789abcdef0123456789abcdef"
	cfg := evidenceauthority.Config{Account: "123456789012", Region: "ap-northeast-1", Table: "evidence", TableID: "fixture-id", LedgerID: id, PrincipalARN: "arn:aws:iam::123456789012:user/fixture"}
	db := &stubDB{}
	authority, err := evidenceauthority.Open(ctx, db, stubIdentity{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(authority, "other", id); err == nil {
		t.Fatal("other table accepted")
	}
	b, err := New(authority, "evidence", id)
	if err != nil {
		t.Fatal(err)
	}
	expected := restoreevidence.Entry{LedgerID: id, OwnerID: "fedcba9876543210fedcba9876543210", OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: "withdrawal_requested"}
	wrong := expected
	wrong.LedgerID = "11111111111111111111111111111111"
	if _, err := b.Acquire(ctx, wrong); err == nil {
		t.Fatal("other ledger accepted")
	}
	lease, err := b.Acquire(ctx, expected)
	if err != nil {
		t.Fatal(err)
	}
	if state, _, err := b.Lookup(ctx, lease, expected); err != nil || state != evidenceflow.Absent {
		t.Fatalf("empty operation lookup: %v %v", state, err)
	}
	db.fail = true
	if state, _, err := b.Lookup(ctx, lease, expected); err == nil || state != evidenceflow.Unknown {
		t.Fatal("read failure not unknown")
	}
	db.fail = false
	stored := expected
	stored.Generation = 1
	stored.Digest = restoreevidence.Digest(stored)
	sv := func(v string) types.AttributeValue { return &types.AttributeValueMemberS{Value: v} }
	db.rows = map[string]map[string]types.AttributeValue{
		"OP#" + id + "#" + stored.OperationID:   {"pk": sv("OP#" + id + "#" + stored.OperationID), "owner_id": sv(stored.OwnerID), "kind": sv(stored.Kind), "policy_version": sv(""), "generation": &types.AttributeValueMemberN{Value: "1"}, "digest": sv(stored.Digest)},
		"EVENT#" + id + "#00000000000000000001": {"pk": sv("EVENT#" + id + "#00000000000000000001"), "ledger_id": sv(id), "owner_id": sv(stored.OwnerID), "operation_id": sv(stored.OperationID), "related_operation_id": sv(""), "policy_version": sv(""), "kind": sv(stored.Kind), "generation": &types.AttributeValueMemberN{Value: "1"}, "previous": sv(""), "digest": sv(stored.Digest)},
	}
	if state, got, err := b.Lookup(ctx, lease, expected); err != nil || state != evidenceflow.Found || got != stored {
		t.Fatal("found receipt not propagated")
	}
	other, _ := New(authority, "evidence", id)
	if _, _, err := other.Lookup(ctx, lease, expected); err == nil {
		t.Fatal("other bridge used session")
	}
}
