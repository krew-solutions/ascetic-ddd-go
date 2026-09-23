package specification

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/seedwork/domain/identity"
	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

type SomethingCriteria struct {
}

func (sc SomethingCriteria) Id() s.FieldNode {
	return s.Field(sc.obj(), "id")
}

func (sc SomethingCriteria) obj() s.ObjectNode {
	return s.Object(s.GlobalScope(), "something")
}

type SomethingSpecification struct {
}

var something = SomethingCriteria{}
var tId, _ = identity.NewIntIdentity(10)
var mId, _ = identity.NewIntIdentity(3)
var sId, _ = identity.NewIntIdentity(3)

func (ss SomethingSpecification) Expression() s.Visitable {
	return s.Equal(
		something.Id(),
		s.Value(MemberSomethingId{
			MemberId{
				TenantId{tId},
				InternalMemberId{mId},
			},
			SomethingId{
				sId,
			},
		}),
	)
}

func (ss SomethingSpecification) Evaluate( /* session session.PgxSession */ ) (
	sql string, params []any, err error,
) {
	return Compile(TestGlobalScopeContext{}, ss.Expression())
}

type MemberSomethingId struct {
	memberId    MemberId
	somethingId SomethingId
}

func (cid MemberSomethingId) Export(ex MemberSomethingIdExporterSetter) {
	ex.SetMemberId(cid.memberId)
	ex.SetSomethingId(cid.somethingId)
}

type MemberSomethingIdExporterSetter interface {
	SetMemberId(MemberId)
	SetSomethingId(SomethingId)
}

type MemberId struct {
	tenantId TenantId
	memberId InternalMemberId
}

func (cid MemberId) Export(ex MemberIdExporterSetter) {
	ex.SetTenantId(cid.tenantId)
	ex.SetMemberId(cid.memberId)
}

type MemberIdExporterSetter interface {
	SetTenantId(TenantId)
	SetMemberId(InternalMemberId)
}

type TenantId struct {
	identity.IntIdentity
}

type InternalMemberId struct {
	identity.IntIdentity
}

type SomethingId struct {
	identity.IntIdentity
}

// Contexts

type SomethingScopeContext struct {
}

func (c SomethingScopeContext) AttrNode(parent s.EmptiableObject, path []string) (Mapped, error) {
	switch path[0] {
	case "id":
		return CompositeExpression(
			CompositeExpression(
				Scalar(s.Field(parent, "tenant_id")),
				Scalar(s.Field(parent, "member_id")),
			),
			Scalar(s.Field(parent, "something_id")),
		), nil
	default:
		return nil, fmt.Errorf("can't get field \"%s\"", path[0])
	}
}

type TestGlobalScopeContext struct {
	something SomethingScopeContext
}

func (c TestGlobalScopeContext) AttrNode(path []string) (Mapped, error) {
	switch path[0] {
	case "something":
		return c.something.AttrNode(s.Object(s.GlobalScope(), "something"), path[1:])
	default:
		return nil, fmt.Errorf("can't get object \"%s\"", path[0])
	}
}

func (c TestGlobalScopeContext) ValueNode(val any) (Mapped, error) {
	switch valTyped := val.(type) {
	case InternalMemberId:
		var ex uint
		valTyped.Export(func(v uint) { ex = v })
		return Scalar(s.Value(ex)), nil
	case TenantId:
		var ex uint
		valTyped.Export(func(v uint) { ex = v })
		return Scalar(s.Value(ex)), nil
	case SomethingId:
		var ex uint
		valTyped.Export(func(v uint) { ex = v })
		return Scalar(s.Value(ex)), nil
	case MemberId:
		var ex MemberIdExporter
		valTyped.Export(&ex)
		nodes := []Mapped{}
		for _, v := range ex.Values() {
			node, err := c.ValueNode(v)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
		}
		return CompositeExpression(nodes...), nil
	case MemberSomethingId:
		var ex MemberSomethingIdExporter
		valTyped.Export(&ex)
		nodes := []Mapped{}
		for _, v := range ex.Values() {
			node, err := c.ValueNode(v)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
		}
		return CompositeExpression(nodes...), nil
	default:
		return nil, fmt.Errorf("can't export \"%#v\"", val)
	}
}

type MemberIdExporter struct {
	values [2]any
}

func (ex MemberIdExporter) Values() []any {
	return ex.values[:]
}

func (ex *MemberIdExporter) SetTenantId(val TenantId) {
	ex.values[0] = val
}

func (ex *MemberIdExporter) SetMemberId(val InternalMemberId) {
	ex.values[1] = val
}

type MemberSomethingIdExporter struct {
	values [2]any
}

func (ex MemberSomethingIdExporter) Values() []any {
	return ex.values[:]
}

func (ex *MemberSomethingIdExporter) SetMemberId(val MemberId) {
	ex.values[0] = val
}

func (ex *MemberSomethingIdExporter) SetSomethingId(val SomethingId) {
	ex.values[1] = val
}

func TestSomethingSpecification(t *testing.T) {
	ss := SomethingSpecification{}
	sql, params, err := ss.Evaluate()
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	assert.Equal(
		t,
		`"something"."tenant_id" = $1 AND "something"."member_id" = $2 AND "something"."something_id" = $3`,
		sql)
	assert.Equal(t, 3, len(params))
	var tIdValue, mIdValue, sIdValue uint
	tId.Export(func(v uint) { tIdValue = v })
	mId.Export(func(v uint) { mIdValue = v })
	sId.Export(func(v uint) { sIdValue = v })

	assert.Equal(t, tIdValue, params[0])
	assert.Equal(t, mIdValue, params[1])
	assert.Equal(t, sIdValue, params[2])
}
