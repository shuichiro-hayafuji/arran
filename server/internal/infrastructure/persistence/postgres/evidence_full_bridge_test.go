package postgres

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/shuichirohayafuji/spendable-today/server/internal/evidenceauthority"
	"github.com/shuichirohayafuji/spendable-today/server/internal/evidencebridge"
	"github.com/shuichirohayafuji/spendable-today/server/internal/evidenceflow"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

type bridgeIdentity struct{}

func (bridgeIdentity) GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	account := "123456789012"
	arn := "arn:aws:iam::123456789012:user/fixture"
	return &sts.GetCallerIdentityOutput{Account: &account, Arn: &arn}, nil
}

type bridgeDB struct {
	mu         sync.Mutex
	rows       map[string]map[string]types.AttributeValue
	loseAppend bool
	writes     int
}

func (*bridgeDB) DescribeTable(context.Context, *dynamodb.DescribeTableInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	arn := "arn:aws:dynamodb:ap-northeast-1:123456789012:table/evidence"
	id := "fixture-id"
	return &dynamodb.DescribeTableOutput{Table: &types.TableDescription{TableArn: &arn, TableId: &id, TableStatus: types.TableStatusActive}}, nil
}
func (d *bridgeDB) GetItem(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if in.ConsistentRead == nil || !*in.ConsistentRead {
		return nil, errors.New("weak read")
	}
	k := in.Key["pk"].(*types.AttributeValueMemberS).Value
	return &dynamodb.GetItemOutput{Item: d.rows[k]}, nil
}
func sattr(v types.AttributeValue) string {
	if v == nil {
		return ""
	}
	return v.(*types.AttributeValueMemberS).Value
}
func (d *bridgeDB) TransactWriteItems(_ context.Context, in *dynamodb.TransactWriteItemsInput, _ ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	// Simulate the specific transaction conditions used by these candidates.
	for _, it := range in.TransactItems {
		if it.Put != nil {
			k := sattr(it.Put.Item["pk"])
			if d.rows[k] != nil {
				return nil, errors.New("duplicate")
			}
		}
		if it.Update != nil {
			k := sattr(it.Update.Key["pk"])
			row := d.rows[k]
			v := it.Update.ExpressionAttributeValues
			if row == nil || row["generation"].(*types.AttributeValueMemberN).Value != v[":old"].(*types.AttributeValueMemberN).Value || sattr(row["digest"]) != sattr(v[":previous"]) {
				return nil, errors.New("head changed")
			}
		}
		if it.ConditionCheck != nil {
			k := sattr(it.ConditionCheck.Key["pk"])
			row := d.rows[k]
			if row == nil || sattr(row["token"]) != sattr(it.ConditionCheck.ExpressionAttributeValues[":token"]) {
				return nil, errors.New("lost lease")
			}
		}
		if it.Delete != nil {
			k := sattr(it.Delete.Key["pk"])
			if sattr(d.rows[k]["token"]) != sattr(it.Delete.ExpressionAttributeValues[":token"]) {
				return nil, errors.New("bad release")
			}
		}
	}
	appended := false
	for _, it := range in.TransactItems {
		if it.Put != nil {
			k := sattr(it.Put.Item["pk"])
			d.rows[k] = it.Put.Item
			if strings.HasPrefix(k, "EVENT#") {
				appended = true
				d.writes++
			}
		}
		if it.Update != nil {
			k := sattr(it.Update.Key["pk"])
			v := it.Update.ExpressionAttributeValues
			d.rows[k]["generation"] = v[":next"]
			d.rows[k]["digest"] = v[":digest"]
		}
		if it.Delete != nil {
			delete(d.rows, sattr(it.Delete.Key["pk"]))
		}
	}
	if appended && d.loseAppend {
		d.loseAppend = false
		return nil, errors.New("response lost")
	}
	return &dynamodb.TransactWriteItemsOutput{}, nil
}
func TestEvidenceJournalBridgeEndToEndFake(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	ctx := identity.WithUser(context.Background(), 701)
	r := openReconciliationRepository(t)
	owner, err := r.activeEvidenceOwner(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ledger := "0123456789abcdef0123456789abcdef"
	in := evidenceIntent{OwnerID: owner, LedgerID: ledger, OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: "withdrawal_requested"}
	if _, err = r.prepareEvidenceIntent(ctx, in); err != nil {
		t.Fatal(err)
	}
	table := "evidence"
	db := &bridgeDB{rows: map[string]map[string]types.AttributeValue{"HEAD#" + ledger: {"pk": &types.AttributeValueMemberS{Value: "HEAD#" + ledger}, "generation": &types.AttributeValueMemberN{Value: strconv.Itoa(0)}, "digest": &types.AttributeValueMemberS{Value: ""}}}, loseAppend: true}
	cfg := evidenceauthority.Config{Account: "123456789012", Region: "ap-northeast-1", Table: table, TableID: "fixture-id", LedgerID: ledger, PrincipalARN: "arn:aws:iam::123456789012:user/fixture"}
	authority, err := evidenceauthority.Open(ctx, db, bridgeIdentity{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	bridge, err := evidencebridge.New(authority, table, ledger)
	if err != nil {
		t.Fatal(err)
	}
	j := evidenceJournal{repo: r}
	if err = evidenceflow.Resume(ctx, j, bridge, owner, in.OperationID); err == nil || db.writes != 1 {
		t.Fatal("ambiguous append accepted")
	}
	// A new journal object emulates process restart; Dynamo remains the authority.
	j = evidenceJournal{repo: r}
	if err = evidenceflow.Resume(ctx, j, bridge, owner, in.OperationID); err != nil || db.writes != 1 {
		t.Fatalf("readback retry %v writes=%d", err, db.writes)
	}
	state, err := j.Load(ctx, owner, in.OperationID)
	if err != nil || !state.Witnessed || state.Attempt == nil {
		t.Fatalf("not witnessed: %+v %v", state, err)
	}
}
