// Package evidenceauthority contains unconnected DynamoDB authority/lease
// building blocks. No application or release path uses this candidate.
package evidenceauthority

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

var ErrUnconfirmed = errors.New("authority or lease unconfirmed")
var idPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

type DB interface {
	DescribeTable(context.Context, *dynamodb.DescribeTableInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
	GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	TransactWriteItems(context.Context, *dynamodb.TransactWriteItemsInput, ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
}
type Identity interface {
	GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}
type Config struct{ Account, Region, Table, TableID, LedgerID, PrincipalARN string }
type Authority struct {
	db  DB
	cfg Config
}

// Matches prevents a bridge from binding this authority to another table or ledger.
func (a *Authority) Matches(table, ledger string) bool {
	return a != nil && a.cfg.Table == table && a.cfg.LedgerID == ledger
}

// Open verifies the caller account and exact table ARN plus stable table ID.
// IAM/endpoint identity and administrative compromise remain deployment concerns.
func Open(ctx context.Context, db DB, stsClient Identity, c Config) (*Authority, error) {
	if db == nil || stsClient == nil || !regexp.MustCompile(`^[0-9]{12}$`).MatchString(c.Account) || !regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(c.Region) || !regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,255}$`).MatchString(c.Table) || c.TableID == "" || c.PrincipalARN == "" || !idPattern.MatchString(c.LedgerID) {
		return nil, ErrUnconfirmed
	}
	who, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil || who == nil || who.Account == nil || *who.Account != c.Account || who.Arn == nil || *who.Arn != c.PrincipalARN {
		return nil, ErrUnconfirmed
	}
	desc, err := db.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: &c.Table})
	if err != nil || desc == nil || desc.Table == nil {
		return nil, ErrUnconfirmed
	}
	table := desc.Table
	want := fmt.Sprintf("arn:aws:dynamodb:%s:%s:table/%s", c.Region, c.Account, c.Table)
	if table.TableArn == nil || *table.TableArn != want || table.TableId == nil || *table.TableId != c.TableID || table.TableStatus != types.TableStatusActive {
		return nil, ErrUnconfirmed
	}
	return &Authority{db: db, cfg: c}, nil
}
func sv(s string) types.AttributeValue { return &types.AttributeValueMemberS{Value: s} }
func key(s string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{"pk": sv(s)}
}

// Lease has no automatic expiry: a crash or ambiguous acquisition blocks until
// a separately audited recovery is designed. Never delete it by timeout alone.
type Lease struct {
	authority *Authority
	token     string
}

func (a *Authority) Acquire(ctx context.Context) (*Lease, error) {
	if a == nil {
		return nil, ErrUnconfirmed
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, ErrUnconfirmed
	}
	token := hex.EncodeToString(raw[:])
	condition := "attribute_not_exists(pk)"
	item := types.TransactWriteItem{Put: &types.Put{TableName: &a.cfg.Table, ConditionExpression: &condition, Item: map[string]types.AttributeValue{"pk": sv("LEASE#" + a.cfg.LedgerID), "token": sv(token)}}}
	out, err := a.db.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: []types.TransactWriteItem{item}})
	if err != nil || out == nil {
		return nil, ErrUnconfirmed
	}
	return &Lease{authority: a, token: token}, nil
}
func (l *Lease) Valid(ctx context.Context) error {
	if l == nil || l.authority == nil {
		return ErrUnconfirmed
	}
	a := l.authority
	yes := true
	out, err := a.db.GetItem(ctx, &dynamodb.GetItemInput{TableName: &a.cfg.Table, Key: key("LEASE#" + a.cfg.LedgerID), ConsistentRead: &yes})
	if err != nil || out == nil || out.Item == nil {
		return ErrUnconfirmed
	}
	v, ok := out.Item["token"].(*types.AttributeValueMemberS)
	if !ok || v.Value != l.token {
		return ErrUnconfirmed
	}
	return nil
}

// fence adds a token condition to the same transaction as the caller's writes.
// Calling Valid alone before a write is not a fence.
func (l *Lease) fence(items []types.TransactWriteItem) ([]types.TransactWriteItem, error) {
	if l == nil || l.authority == nil || len(items) == 0 || len(items) >= 100 {
		return nil, ErrUnconfirmed
	}
	a := l.authority
	condition := "#token = :token"
	copyItems := append([]types.TransactWriteItem(nil), items...)
	return append(copyItems, types.TransactWriteItem{ConditionCheck: &types.ConditionCheck{TableName: &a.cfg.Table, Key: key("LEASE#" + a.cfg.LedgerID), ConditionExpression: &condition, ExpressionAttributeNames: map[string]string{"#token": "token"}, ExpressionAttributeValues: map[string]types.AttributeValue{":token": sv(l.token)}}}), nil
}
func (l *Lease) Release(ctx context.Context) error {
	if l == nil || l.authority == nil {
		return ErrUnconfirmed
	}
	a := l.authority
	condition := "#token = :token"
	item := types.TransactWriteItem{Delete: &types.Delete{TableName: &a.cfg.Table, Key: key("LEASE#" + a.cfg.LedgerID), ConditionExpression: &condition, ExpressionAttributeNames: map[string]string{"#token": "token"}, ExpressionAttributeValues: map[string]types.AttributeValue{":token": sv(l.token)}}}
	out, err := a.db.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: []types.TransactWriteItem{item}})
	if err != nil || out == nil {
		return ErrUnconfirmed
	}
	return nil
}

// FencedClient can be passed to the unconnected DynamoStore candidate. Every
// write adds the lease check to the same DynamoDB transaction. This does not
// constrain any other credentials or clients that can bypass this wrapper.
type FencedClient struct{ lease *Lease }

func (f *FencedClient) permittedKey(values map[string]types.AttributeValue) bool {
	if len(values) != 1 {
		return false
	}
	v, ok := values["pk"].(*types.AttributeValueMemberS)
	if !ok {
		return false
	}
	id := f.lease.authority.cfg.LedgerID
	if v.Value == "HEAD#"+id {
		return true
	}
	for _, prefix := range []string{"EVENT#" + id + "#", "OP#" + id + "#", "FINISH#" + id + "#"} {
		if strings.HasPrefix(v.Value, prefix) && len(v.Value) > len(prefix) {
			return true
		}
	}
	return false
}

func (l *Lease) Client() *FencedClient { return &FencedClient{lease: l} }
func (f *FencedClient) GetItem(ctx context.Context, in *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if f == nil || f.lease == nil || in == nil || in.TableName == nil || *in.TableName != f.lease.authority.cfg.Table || !f.permittedKey(in.Key) || in.ConsistentRead == nil || !*in.ConsistentRead || f.lease.Valid(ctx) != nil {
		return nil, ErrUnconfirmed
	}
	return f.lease.authority.db.GetItem(ctx, in, opts...)
}
func (f *FencedClient) TransactWriteItems(ctx context.Context, in *dynamodb.TransactWriteItemsInput, opts ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	if f == nil || f.lease == nil || in == nil {
		return nil, ErrUnconfirmed
	}
	for _, item := range in.TransactItems {
		var table *string
		var keyValues map[string]types.AttributeValue
		switch {
		case item.Put != nil && item.Update == nil && item.Delete == nil && item.ConditionCheck == nil:
			table = item.Put.TableName
			keyValues = map[string]types.AttributeValue{"pk": item.Put.Item["pk"]}
		case item.Update != nil && item.Put == nil && item.Delete == nil && item.ConditionCheck == nil:
			table = item.Update.TableName
			keyValues = item.Update.Key
		case item.ConditionCheck != nil && item.Put == nil && item.Update == nil && item.Delete == nil:
			table = item.ConditionCheck.TableName
			keyValues = item.ConditionCheck.Key
		default:
			return nil, ErrUnconfirmed
		}
		if table == nil || *table != f.lease.authority.cfg.Table || !f.permittedKey(keyValues) {
			return nil, ErrUnconfirmed
		}
	}
	items, err := f.lease.fence(in.TransactItems)
	if err != nil {
		return nil, ErrUnconfirmed
	}
	copyInput := *in
	copyInput.TransactItems = items
	out, err := f.lease.authority.db.TransactWriteItems(ctx, &copyInput, opts...)
	if err != nil || out == nil {
		return nil, ErrUnconfirmed
	}
	return out, nil
}
