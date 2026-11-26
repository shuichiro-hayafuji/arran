package evidenceauthority

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"testing"
)

type fakeSTS struct {
	account string
	err     error
}

func (f fakeSTS) GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	arn := "arn:aws:iam::" + f.account + ":user/fixture"
	return &sts.GetCallerIdentityOutput{Account: &f.account, Arn: &arn}, nil
}

type fakeDB struct {
	arn, tableID             string
	token                    string
	loseAcquire, loseRelease bool
	writes                   int
}

func (f *fakeDB) DescribeTable(context.Context, *dynamodb.DescribeTableInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	return &dynamodb.DescribeTableOutput{Table: &types.TableDescription{TableArn: &f.arn, TableId: &f.tableID, TableStatus: types.TableStatusActive}}, nil
}
func (f *fakeDB) GetItem(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if in.ConsistentRead == nil || !*in.ConsistentRead {
		return nil, errors.New("not strongly consistent")
	}
	if f.token == "" {
		return &dynamodb.GetItemOutput{}, nil
	}
	return &dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{"token": sv(f.token)}}, nil
}
func (f *fakeDB) TransactWriteItems(_ context.Context, in *dynamodb.TransactWriteItemsInput, _ ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	if len(in.TransactItems) > 1 {
		check := in.TransactItems[len(in.TransactItems)-1].ConditionCheck
		if check == nil || *check.ConditionExpression != "#token = :token" || check.ExpressionAttributeValues[":token"].(*types.AttributeValueMemberS).Value != f.token || f.token == "" {
			return nil, errors.New("stale lease")
		}
		f.writes++
		return &dynamodb.TransactWriteItemsOutput{}, nil
	}
	if len(in.TransactItems) != 1 {
		return nil, errors.New("unexpected transaction")
	}
	item := in.TransactItems[0]
	if item.Put != nil {
		if *item.Put.ConditionExpression != "attribute_not_exists(pk)" || f.token != "" {
			return nil, errors.New("conditional failed")
		}
		f.token = item.Put.Item["token"].(*types.AttributeValueMemberS).Value
		if f.loseAcquire {
			return nil, errors.New("response lost")
		}
		return &dynamodb.TransactWriteItemsOutput{}, nil
	}
	if item.Delete != nil {
		v := item.Delete.ExpressionAttributeValues[":token"].(*types.AttributeValueMemberS).Value
		if *item.Delete.ConditionExpression != "#token = :token" || f.token != v {
			return nil, errors.New("conditional failed")
		}
		f.token = ""
		if f.loseRelease {
			return nil, errors.New("response lost")
		}
		return &dynamodb.TransactWriteItemsOutput{}, nil
	}
	return nil, errors.New("unexpected item")
}
func TestAuthorityAndLeaseFailClosed(t *testing.T) {
	ctx := context.Background()
	c := Config{Account: "123456789012", Region: "ap-northeast-1", Table: "evidence", TableID: "table-identity", LedgerID: "0123456789abcdef0123456789abcdef", PrincipalARN: "arn:aws:iam::123456789012:user/fixture"}
	db := &fakeDB{arn: "arn:aws:dynamodb:ap-northeast-1:123456789012:table/evidence", tableID: c.TableID}
	if _, err := Open(ctx, db, fakeSTS{account: "000000000000"}, c); err == nil {
		t.Fatal("wrong account accepted")
	}
	wrong := *db
	wrong.tableID = "replacement"
	if _, err := Open(ctx, &wrong, fakeSTS{account: c.Account}, c); err == nil {
		t.Fatal("recreated table accepted")
	}
	a, err := Open(ctx, db, fakeSTS{account: c.Account}, c)
	if err != nil {
		t.Fatal(err)
	}
	l, err := a.Acquire(ctx)
	if err != nil || l.Valid(ctx) != nil {
		t.Fatal("lease not acquired")
	}
	if _, err = a.Acquire(ctx); err == nil {
		t.Fatal("concurrent lease accepted")
	}
	items, err := l.fence([]types.TransactWriteItem{{Put: &types.Put{TableName: &c.Table, Item: key("HEAD#" + c.LedgerID)}}})
	if err != nil || len(items) != 2 || items[1].ConditionCheck == nil {
		t.Fatal("missing atomic fence")
	}
	check := items[1].ConditionCheck
	if *check.ConditionExpression != "#token = :token" || check.Key["pk"].(*types.AttributeValueMemberS).Value != "LEASE#"+c.LedgerID || check.ExpressionAttributeValues[":token"].(*types.AttributeValueMemberS).Value != db.token {
		t.Fatal("fence is not bound to lease")
	}
	client := l.Client()
	otherLedger := "11111111111111111111111111111111"
	yes := true
	for _, keyName := range []string{"HEAD#" + otherLedger, "EVENT#" + otherLedger + "#00000000000000000001", "OP#" + otherLedger + "#aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "FINISH#" + otherLedger + "#aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		if _, err := client.GetItem(ctx, &dynamodb.GetItemInput{TableName: &c.Table, Key: key(keyName), ConsistentRead: &yes}); err == nil {
			t.Fatal("other ledger read accepted")
		}
		if _, err := client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: []types.TransactWriteItem{{Put: &types.Put{TableName: &c.Table, Item: key(keyName)}}}}); err == nil {
			t.Fatal("other ledger write accepted")
		}
	}
	if _, err := client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: []types.TransactWriteItem{{Put: &types.Put{TableName: &c.Table, Item: key("HEAD#" + c.LedgerID)}}}}); err != nil || db.writes != 1 {
		t.Fatal("valid fenced write failed")
	}
	otherTable := "other"
	if _, err := client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: []types.TransactWriteItem{{Put: &types.Put{TableName: &otherTable}}}}); err == nil || db.writes != 1 {
		t.Fatal("other table accepted")
	}
	if _, err := client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: []types.TransactWriteItem{{Delete: &types.Delete{TableName: &c.Table}}}}); err == nil || db.writes != 1 {
		t.Fatal("arbitrary deletion accepted")
	}
	if err = l.Release(ctx); err != nil {
		t.Fatal(err)
	}
	if l.Valid(ctx) == nil {
		t.Fatal("released lease valid")
	}
	if _, err := client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: []types.TransactWriteItem{{Put: &types.Put{TableName: &c.Table, Item: key("HEAD#" + c.LedgerID)}}}}); err == nil || db.writes != 1 {
		t.Fatal("stale fenced write succeeded")
	}
	db.loseAcquire = true
	if _, err = a.Acquire(ctx); err == nil || db.token == "" {
		t.Fatal("ambiguous acquisition accepted")
	}
	db.loseAcquire = false
	if _, err = a.Acquire(ctx); err == nil {
		t.Fatal("ambiguous acquisition created second holder")
	}
}
