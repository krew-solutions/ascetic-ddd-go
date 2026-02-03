package public

import (
	"testing"
	"time"

	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

// TestDelegating tests the Delegating adapter
func TestDelegating(t *testing.T) {
	t.Run("Creation", func(t *testing.T) {
		valueNode := s.Value(42)
		delegating := NewDelegating(valueNode)
		if delegating.Delegate() != valueNode {
			t.Errorf("Expected delegate to be %v, got %v", valueNode, delegating.Delegate())
		}
	})

	t.Run("DelegateReturnsVisitable", func(t *testing.T) {
		valueNode := s.Value("test")
		delegating := NewDelegating(valueNode)
		delegate := delegating.Delegate()
		_, ok := delegate.(s.Visitable)
		if !ok {
			t.Error("Expected delegate to be Visitable")
		}
	})
}

// TestLogical tests the Logical adapter
func TestLogical(t *testing.T) {
	t.Run("AndOperation", func(t *testing.T) {
		left := NewLogical(s.Value(true))
		right := NewLogical(s.Value(false))
		result := left.And(right)

		_, ok := result.(Logical)
		if !ok {
			t.Error("Expected result to implement Logical")
		}

		delegate := result.Delegate()
		_, ok = delegate.(s.InfixNode)
		if !ok {
			t.Error("Expected delegate to be InfixNode (And)")
		}
		if delegate.(s.InfixNode).Operator() != s.OperatorAnd {
			t.Error("Expected AND operator")
		}
	})

	t.Run("OrOperation", func(t *testing.T) {
		left := NewLogical(s.Value(true))
		right := NewLogical(s.Value(false))
		result := left.Or(right)

		_, ok := result.(Logical)
		if !ok {
			t.Error("Expected result to implement Logical")
		}

		delegate := result.Delegate()
		_, ok = delegate.(s.InfixNode)
		if !ok {
			t.Error("Expected delegate to be InfixNode (Or)")
		}
		if delegate.(s.InfixNode).Operator() != s.OperatorOr {
			t.Error("Expected OR operator")
		}
	})

	t.Run("IsOperation", func(t *testing.T) {
		left := NewLogical(s.Value(true))
		right := NewLogical(s.Value(true))
		result := left.Is(right)

		_, ok := result.(Logical)
		if !ok {
			t.Error("Expected result to implement Logical")
		}

		delegate := result.Delegate()
		_, ok = delegate.(s.InfixNode)
		if !ok {
			t.Error("Expected delegate to be InfixNode (Is)")
		}
		if delegate.(s.InfixNode).Operator() != s.OperatorIs {
			t.Error("Expected IS operator")
		}
	})
}

// TestNullable tests the Nullable adapter
func TestNullable(t *testing.T) {
	t.Run("IsNull", func(t *testing.T) {
		nullable := NewNullable(s.Value(nil))
		result := nullable.IsNull()

		_, ok := result.(Logical)
		if !ok {
			t.Error("Expected result to implement Logical")
		}

		delegate := result.Delegate()
		_, ok = delegate.(s.PostfixNode)
		if !ok {
			t.Error("Expected delegate to be PostfixNode (IsNull)")
		}
		if delegate.(s.PostfixNode).Operator() != s.OperatorIsNull {
			t.Error("Expected IS NULL operator")
		}
	})

	t.Run("IsNotNull", func(t *testing.T) {
		nullable := NewNullable(s.Value(42))
		result := nullable.IsNotNull()

		_, ok := result.(Logical)
		if !ok {
			t.Error("Expected result to implement Logical")
		}

		delegate := result.Delegate()
		_, ok = delegate.(s.PostfixNode)
		if !ok {
			t.Error("Expected delegate to be PostfixNode (IsNotNull)")
		}
		if delegate.(s.PostfixNode).Operator() != s.OperatorIsNotNull {
			t.Error("Expected IS NOT NULL operator")
		}
	})
}

// TestComparison tests the Comparison adapter
func TestComparison(t *testing.T) {
	t.Run("Equal", func(t *testing.T) {
		left := NewComparison(s.Value(5))
		right := NewComparison(s.Value(5))
		result := left.Eq(right)

		_, ok := result.(Logical)
		if !ok {
			t.Error("Expected result to implement Logical")
		}

		delegate := result.Delegate()
		_, ok = delegate.(s.InfixNode)
		if !ok {
			t.Error("Expected delegate to be InfixNode (Equal)")
		}
		if delegate.(s.InfixNode).Operator() != s.OperatorEq {
			t.Error("Expected EQ operator")
		}
	})

	t.Run("NotEqual", func(t *testing.T) {
		left := NewComparison(s.Value(5))
		right := NewComparison(s.Value(10))
		result := left.Ne(right)

		delegate := result.Delegate()
		_, ok := delegate.(s.InfixNode)
		if !ok {
			t.Error("Expected delegate to be InfixNode (NotEqual)")
		}
		if delegate.(s.InfixNode).Operator() != s.OperatorNe {
			t.Error("Expected NE operator")
		}
	})

	t.Run("GreaterThan", func(t *testing.T) {
		left := NewComparison(s.Value(10))
		right := NewComparison(s.Value(5))
		result := left.Gt(right)

		delegate := result.Delegate()
		if delegate.(s.InfixNode).Operator() != s.OperatorGt {
			t.Error("Expected GT operator")
		}
	})

	t.Run("LessThan", func(t *testing.T) {
		left := NewComparison(s.Value(5))
		right := NewComparison(s.Value(10))
		result := left.Lt(right)

		delegate := result.Delegate()
		if delegate.(s.InfixNode).Operator() != s.OperatorLt {
			t.Error("Expected LT operator")
		}
	})

	t.Run("GreaterThanEqual", func(t *testing.T) {
		left := NewComparison(s.Value(10))
		right := NewComparison(s.Value(10))
		result := left.Gte(right)

		delegate := result.Delegate()
		if delegate.(s.InfixNode).Operator() != s.OperatorGte {
			t.Error("Expected GTE operator")
		}
	})

	t.Run("LessThanEqual", func(t *testing.T) {
		left := NewComparison(s.Value(5))
		right := NewComparison(s.Value(5))
		result := left.Lte(right)

		delegate := result.Delegate()
		if delegate.(s.InfixNode).Operator() != s.OperatorLte {
			t.Error("Expected LTE operator")
		}
	})

	t.Run("LeftShift", func(t *testing.T) {
		left := NewComparison(s.Value(5))
		right := NewComparison(s.Value(1))
		result := left.Lshift(right)

		delegate := result.Delegate()
		if delegate.(s.InfixNode).Operator() != s.OperatorLshift {
			t.Error("Expected LSHIFT operator")
		}
	})

	t.Run("RightShift", func(t *testing.T) {
		left := NewComparison(s.Value(5))
		right := NewComparison(s.Value(1))
		result := left.Rshift(right)

		delegate := result.Delegate()
		if delegate.(s.InfixNode).Operator() != s.OperatorRshift {
			t.Error("Expected RSHIFT operator")
		}
	})
}

// TestMathematical tests the Mathematical adapter
func TestMathematical(t *testing.T) {
	t.Run("Add", func(t *testing.T) {
		left := NewMathematical(s.Value(5))
		right := NewMathematical(s.Value(3))
		result := left.Add(right)

		_, ok := result.(Mathematical)
		if !ok {
			t.Error("Expected result to implement Mathematical")
		}

		delegate := result.Delegate()
		if delegate.(s.InfixNode).Operator() != s.OperatorAdd {
			t.Error("Expected ADD operator")
		}
	})

	t.Run("Sub", func(t *testing.T) {
		left := NewMathematical(s.Value(5))
		right := NewMathematical(s.Value(3))
		result := left.Sub(right)

		delegate := result.Delegate()
		if delegate.(s.InfixNode).Operator() != s.OperatorSub {
			t.Error("Expected SUB operator")
		}
	})

	t.Run("Mul", func(t *testing.T) {
		left := NewMathematical(s.Value(5))
		right := NewMathematical(s.Value(3))
		result := left.Mul(right)

		delegate := result.Delegate()
		if delegate.(s.InfixNode).Operator() != s.OperatorMul {
			t.Error("Expected MUL operator")
		}
	})

	t.Run("Div", func(t *testing.T) {
		left := NewMathematical(s.Value(6))
		right := NewMathematical(s.Value(3))
		result := left.Div(right)

		delegate := result.Delegate()
		if delegate.(s.InfixNode).Operator() != s.OperatorDiv {
			t.Error("Expected DIV operator")
		}
	})

	t.Run("Mod", func(t *testing.T) {
		left := NewMathematical(s.Value(5))
		right := NewMathematical(s.Value(3))
		result := left.Mod(right)

		delegate := result.Delegate()
		if delegate.(s.InfixNode).Operator() != s.OperatorMod {
			t.Error("Expected MOD operator")
		}
	})
}

// TestDatatypes tests datatype classes
func TestDatatypes(t *testing.T) {
	t.Run("BooleanInheritance", func(t *testing.T) {
		boolean := NewBoolean(s.Value(true))
		// Check that boolean implements Logical
		var _ Logical = boolean
		// Check that it embeds *LogicalImp
		if boolean.LogicalImp == nil {
			t.Error("Expected Boolean to embed *LogicalImp")
		}
	})

	t.Run("NullBooleanInheritance", func(t *testing.T) {
		nullBoolean := NewNullBoolean(s.Value(nil))
		// Check embedding
		if nullBoolean.Boolean == nil {
			t.Error("Expected NullBoolean to embed *Boolean")
		}
		// Check that it implements Nullable
		var _ Nullable = nullBoolean
	})

	t.Run("NumberInheritance", func(t *testing.T) {
		number := NewNumber(s.Value(42))
		// Check that number implements both interfaces
		var _ Comparison = number
		var _ Mathematical = number
		// Check DelegatingImp embedding
		if number.DelegatingImp == nil {
			t.Error("Expected Number to embed *DelegatingImp")
		}
	})

	t.Run("NullNumberInheritance", func(t *testing.T) {
		nullNumber := NewNullNumber(s.Value(nil))
		if nullNumber.Number == nil {
			t.Error("Expected NullNumber to embed *Number")
		}
		// Check that it implements Nullable
		var _ Nullable = nullNumber
	})

	t.Run("DatetimeInheritance", func(t *testing.T) {
		dt := NewDatetime(s.Value(time.Now()))
		// Check that datetime implements both interfaces
		var _ Comparison = dt
		var _ Mathematical = dt
		if dt.DelegatingImp == nil {
			t.Error("Expected Datetime to embed *DelegatingImp")
		}
	})

	t.Run("NullDatetimeInheritance", func(t *testing.T) {
		nullDt := NewNullDatetime(s.Value(nil))
		if nullDt.Datetime == nil {
			t.Error("Expected NullDatetime to embed *Datetime")
		}
		// Check that it implements Nullable
		var _ Nullable = nullDt
	})

	t.Run("TextInheritance", func(t *testing.T) {
		text := NewText(s.Value("hello"))
		// Check that text implements Comparison
		var _ Comparison = text
		if text.ComparisonImp == nil {
			t.Error("Expected Text to embed *ComparisonImp")
		}
	})

	t.Run("NullTextInheritance", func(t *testing.T) {
		nullText := NewNullText(s.Value(nil))
		if nullText.Text == nil {
			t.Error("Expected NullText to embed *Text")
		}
		// Check that it implements Nullable
		var _ Nullable = nullText
	})
}

// TestFieldFactory tests MakeField constructors
func TestFieldFactory(t *testing.T) {
	t.Run("BooleanFieldCreation", func(t *testing.T) {
		bf := MakeBooleanField("is_active")
		if bf == nil {
			t.Error("Expected non-nil Boolean")
		}
		delegate := bf.Delegate()
		_, ok := delegate.(s.FieldNode)
		if !ok {
			t.Error("Expected delegate to be FieldNode")
		}
	})

	t.Run("NullBooleanFieldCreation", func(t *testing.T) {
		nbf := MakeNullBooleanField("is_deleted")
		if nbf == nil {
			t.Error("Expected non-nil NullBoolean")
		}
		delegate := nbf.Delegate()
		_, ok := delegate.(s.FieldNode)
		if !ok {
			t.Error("Expected delegate to be FieldNode")
		}
	})

	t.Run("NumberFieldCreation", func(t *testing.T) {
		nf := MakeNumberField("age")
		if nf == nil {
			t.Error("Expected non-nil Number")
		}
		delegate := nf.Delegate()
		_, ok := delegate.(s.FieldNode)
		if !ok {
			t.Error("Expected delegate to be FieldNode")
		}
	})

	t.Run("NullNumberFieldCreation", func(t *testing.T) {
		nnf := MakeNullNumberField("score")
		if nnf == nil {
			t.Error("Expected non-nil NullNumber")
		}
		delegate := nnf.Delegate()
		_, ok := delegate.(s.FieldNode)
		if !ok {
			t.Error("Expected delegate to be FieldNode")
		}
	})

	t.Run("DatetimeFieldCreation", func(t *testing.T) {
		df := MakeDatetimeField("created_at")
		if df == nil {
			t.Error("Expected non-nil Datetime")
		}
		delegate := df.Delegate()
		_, ok := delegate.(s.FieldNode)
		if !ok {
			t.Error("Expected delegate to be FieldNode")
		}
	})

	t.Run("NullDatetimeFieldCreation", func(t *testing.T) {
		ndf := MakeNullDatetimeField("deleted_at")
		if ndf == nil {
			t.Error("Expected non-nil NullDatetime")
		}
		delegate := ndf.Delegate()
		_, ok := delegate.(s.FieldNode)
		if !ok {
			t.Error("Expected delegate to be FieldNode")
		}
	})

	t.Run("TextFieldCreation", func(t *testing.T) {
		tf := MakeTextField("name")
		if tf == nil {
			t.Error("Expected non-nil Text")
		}
		delegate := tf.Delegate()
		_, ok := delegate.(s.FieldNode)
		if !ok {
			t.Error("Expected delegate to be FieldNode")
		}
	})

	t.Run("NullTextFieldCreation", func(t *testing.T) {
		ntf := MakeNullTextField("description")
		if ntf == nil {
			t.Error("Expected non-nil NullText")
		}
		delegate := ntf.Delegate()
		_, ok := delegate.(s.FieldNode)
		if !ok {
			t.Error("Expected delegate to be FieldNode")
		}
	})
}

// TestValueFactory tests MakeValue constructors
func TestValueFactory(t *testing.T) {
	t.Run("BooleanValueCreation", func(t *testing.T) {
		bv := MakeBooleanValue(true)
		if bv == nil {
			t.Error("Expected non-nil Boolean")
		}
		delegate := bv.Delegate()
		_, ok := delegate.(s.ValueNode)
		if !ok {
			t.Error("Expected delegate to be ValueNode")
		}
	})

	t.Run("NumberValueCreation", func(t *testing.T) {
		nv := MakeNumberValue(42)
		if nv == nil {
			t.Error("Expected non-nil Number")
		}
		delegate := nv.Delegate()
		_, ok := delegate.(s.ValueNode)
		if !ok {
			t.Error("Expected delegate to be ValueNode")
		}
	})

	t.Run("DatetimeValueCreation", func(t *testing.T) {
		now := time.Now()
		dv := MakeDatetimeValue(now)
		if dv == nil {
			t.Error("Expected non-nil Datetime")
		}
		delegate := dv.Delegate()
		_, ok := delegate.(s.ValueNode)
		if !ok {
			t.Error("Expected delegate to be ValueNode")
		}
	})

	t.Run("TextValueCreation", func(t *testing.T) {
		tv := MakeTextValue("hello")
		if tv == nil {
			t.Error("Expected non-nil Text")
		}
		delegate := tv.Delegate()
		_, ok := delegate.(s.ValueNode)
		if !ok {
			t.Error("Expected delegate to be ValueNode")
		}
	})
}

// TestHelperFunctions tests helper functions
func TestHelperFunctions(t *testing.T) {
	t.Run("ObjectSimpleName", func(t *testing.T) {
		obj := Object_("user")
		// obj is already s.ObjectNode type by function signature
		if obj.Name() != "user" {
			t.Errorf("Expected name 'user', got '%s'", obj.Name())
		}
	})

	t.Run("ObjectDottedName", func(t *testing.T) {
		obj := Object_("user.profile")
		if obj.Name() != "profile" {
			t.Errorf("Expected name 'profile', got '%s'", obj.Name())
		}
		parent := obj.Parent()
		// Check if parent is an ObjectNode (not GlobalScope)
		if parent.IsRoot() {
			t.Error("Expected parent to not be root")
		}
		// Parent must be ObjectNode, assert to it
		parentObj := parent.(s.ObjectNode)
		if parentObj.Name() != "user" {
			t.Errorf("Expected parent name 'user', got '%s'", parentObj.Name())
		}
	})

	t.Run("ObjectMultipleDots", func(t *testing.T) {
		obj := Object_("root.user.profile")
		if obj.Name() != "profile" {
			t.Errorf("Expected name 'profile', got '%s'", obj.Name())
		}
	})

	t.Run("FieldSimpleName", func(t *testing.T) {
		f := Field("name")
		// f is already s.FieldNode by function signature
		if f.Name() != "name" {
			t.Errorf("Expected field name 'name', got '%s'", f.Name())
		}
		parent := f.Object()
		// Check if parent is GlobalScope
		if !parent.IsRoot() {
			t.Error("Expected parent to be GlobalScopeNode")
		}
	})

	t.Run("FieldDottedName", func(t *testing.T) {
		f := Field("user.name")
		if f.Name() != "name" {
			t.Errorf("Expected field name 'name', got '%s'", f.Name())
		}
		parent := f.Object()
		if parent.IsRoot() {
			t.Error("Expected parent to not be root")
		}
		parentObj := parent.(s.ObjectNode)
		if parentObj.Name() != "user" {
			t.Errorf("Expected parent name 'user', got '%s'", parentObj.Name())
		}
	})

	t.Run("FieldMultipleDots", func(t *testing.T) {
		f := Field("root.user.name")
		if f.Name() != "name" {
			t.Errorf("Expected field name 'name', got '%s'", f.Name())
		}
		parent := f.Object()
		if parent.IsRoot() {
			t.Error("Expected parent to not be root")
		}
		parentObj := parent.(s.ObjectNode)
		if parentObj.Name() != "user" {
			t.Errorf("Expected parent name 'user', got '%s'", parentObj.Name())
		}
	})
}

// TestIntegration tests integration scenarios
func TestIntegration(t *testing.T) {
	t.Run("FieldComparison", func(t *testing.T) {
		age := MakeNumberField("age")
		minAge := MakeNumberValue(18)

		result := age.Gt(minAge)
		if result == nil {
			t.Error("Expected non-nil result")
		}
		delegate := result.Delegate()
		if delegate.(s.InfixNode).Operator() != s.OperatorGt {
			t.Error("Expected GT operator")
		}
	})

	t.Run("FieldLogicalOperations", func(t *testing.T) {
		age := MakeNumberField("age")
		isActive := MakeBooleanField("is_active")

		ageCheck := age.Gte(MakeNumberValue(18))
		combined := ageCheck.And(isActive)

		if combined == nil {
			t.Error("Expected non-nil result")
		}
		delegate := combined.Delegate()
		if delegate.(s.InfixNode).Operator() != s.OperatorAnd {
			t.Error("Expected AND operator")
		}
	})

	t.Run("NullableFieldOperations", func(t *testing.T) {
		email := MakeNullTextField("email")

		isNullCheck := email.IsNull()
		isNotNullCheck := email.IsNotNull()

		if isNullCheck == nil {
			t.Error("Expected non-nil result for IsNull")
		}
		if isNotNullCheck == nil {
			t.Error("Expected non-nil result for IsNotNull")
		}
	})

	t.Run("ComplexExpression", func(t *testing.T) {
		age := MakeNumberField("age")
		name := MakeTextField("name")
		isActive := MakeBooleanField("is_active")

		// (age > 18) AND (name == "Alice") AND is_active
		ageCheck := age.Gt(MakeNumberValue(18))
		nameCheck := name.Eq(MakeTextValue("Alice"))
		expression := ageCheck.And(nameCheck).And(isActive)

		if expression == nil {
			t.Error("Expected non-nil result")
		}
	})

	t.Run("MathematicalOperations", func(t *testing.T) {
		price := MakeNumberField("price")
		quantity := MakeNumberField("quantity")
		discount := MakeNumberField("discount")

		// (price * quantity) - discount
		total := price.Mul(quantity).Sub(discount)

		// Should return Mathematical type
		if total == nil {
			t.Error("Expected non-nil result")
		}
		delegate := total.Delegate()
		if delegate.(s.InfixNode).Operator() != s.OperatorSub {
			t.Error("Expected SUB operator at top level")
		}
	})

	t.Run("ShiftOperations", func(t *testing.T) {
		value := MakeNumberField("value")
		shiftAmount := MakeNumberValue(2)

		leftShifted := value.Lshift(shiftAmount)
		rightShifted := value.Rshift(shiftAmount)

		if leftShifted == nil {
			t.Error("Expected non-nil result for Lshift")
		}
		if rightShifted == nil {
			t.Error("Expected non-nil result for Rshift")
		}
	})

	t.Run("ModuloOperation", func(t *testing.T) {
		number := MakeNumberField("number")
		divisor := MakeNumberValue(10)

		remainder := number.Mod(divisor)

		// Should return Mathematical type
		if remainder == nil {
			t.Error("Expected non-nil result")
		}
		delegate := remainder.Delegate()
		if delegate.(s.InfixNode).Operator() != s.OperatorMod {
			t.Error("Expected MOD operator")
		}
	})
}
