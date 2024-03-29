package bsonschema

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/exp/constraints"
)

type numbers[T Specification, N constraints.Integer | constraints.Float | primitive.Decimal128] interface {
	SetExclusiveMaximum(n N) T
	SetExclusiveMinimum(n N) T
	SetMaximum(n N) T
	SetMinimum(n N) T
	SetMultipleOf(n N) T
}

var (
	_ numbers[Decimal, primitive.Decimal128] = Decimal(nil)
	_ numbers[Int, int32]                    = Int(nil)
	_ numbers[Long, int64]                   = Long(nil)
	_ numbers[Double, float64]               = Double(nil)
)

func TestNewNull_sharedMethods(t *testing.T) {
	testSharedMethods(t, NewNull)
}

func TestNewObject_sharedMethods(t *testing.T) {
	testSharedMethods(t, NewObject)
}

func TestNewArray_sharedMethods(t *testing.T) {
	testSharedMethods(t, NewArray)
}

func TestNewObjectID_sharedMethods(t *testing.T) {
	testSharedMethods(t, NewObjectID)
}

func TestNewString_sharedMethods(t *testing.T) {
	testSharedMethods(t, NewString)
}

func TestNewRegex_sharedMethods(t *testing.T) {
	testSharedMethods(t, NewRegex)
}

func TestNewUUID_sharedMethods(t *testing.T) {
	testSharedMethods(t, NewUUID)
}

func TestNewBinData_sharedMethods(t *testing.T) {
	testSharedMethods(t, NewBinData)
}

func TestNewTimestamp_sharedMethods(t *testing.T) {
	testSharedMethods(t, NewTimestamp)
}

func TestNewDate_sharedMethods(t *testing.T) {
	testSharedMethods(t, NewDate)
}

func TestNewBool_sharedMethods(t *testing.T) {
	testSharedMethods(t, NewBool)
}

func TestNewBoolean_sharedMethods(t *testing.T) {
	testSharedMethods(t, NewBoolean)
}

func TestNewDouble_sharedMethods(t *testing.T) {
	testSharedMethods(t, NewDouble)
}

func TestNewInt_sharedMethods(t *testing.T) {
	testSharedMethods(t, NewInt)
}

func TestNewLong_sharedMethods(t *testing.T) {
	testSharedMethods(t, NewLong)
}

func TestNewDecimal_sharedMethods(t *testing.T) {
	testSharedMethods(t, NewDecimal)
}

func testSharedMethods[T sharedInterface[T]](t *testing.T, newT func() T) {
	t.Run("SetDescription", func(t *testing.T) {
		s := newT()
		r := s.SetDescription("some description")
		assert.Len(t, s, 1)
		assert.Len(t, r, 2)
		assert.True(t, slices.ContainsFunc(r, func(e bson.E) bool {
			return e.Key == "description"
		}))
	})
	t.Run("SetEnum", func(t *testing.T) {
		s := newT()
		r := s.SetEnum(bson.A{})
		assert.Len(t, s, 1)
		assert.Len(t, r, 2)
		assert.True(t, slices.ContainsFunc(r, func(e bson.E) bool {
			return e.Key == "enum"
		}))
	})
	t.Run("AllOf", func(t *testing.T) {
		s := newT()
		r := s.AllOf([]Specification{})
		assert.Len(t, s, 1)
		assert.True(t, slices.ContainsFunc(r, func(e bson.E) bool {
			return e.Key == "allOf"
		}))
	})
	t.Run("AnyOf", func(t *testing.T) {
		s := newT()
		r := s.AnyOf([]Specification{})
		assert.Len(t, s, 1)
		assert.True(t, slices.ContainsFunc(r, func(e bson.E) bool {
			return e.Key == "anyOf"
		}))
	})
	t.Run("Not", func(t *testing.T) {
		s := newT()
		r := s.Not(newT())
		assert.Len(t, s, 1)
		assert.True(t, slices.ContainsFunc(r, func(e bson.E) bool {
			return e.Key == "not"
		}))
	})
	t.Run("OneOf", func(t *testing.T) {
		s := newT()
		r := s.OneOf([]Specification{})
		assert.Len(t, s, 1)
		assert.True(t, slices.ContainsFunc(r, func(e bson.E) bool {
			return e.Key == "oneOf"
		}))
	})
}

func TestObject_allowAdditionalFields(t *testing.T) {
	assert.True(t, NewObject().additionalProperties())
	assert.True(t, NewObject().SetAdditionalProperties(true).additionalProperties())
	assert.False(t, NewObject().SetAdditionalProperties(false).additionalProperties())

	assert.NotPanics(t, func() {
		Object(bson.D{{Key: "additionalProperties", Value: "banana"}}).additionalProperties()
	})
}

func TestObject_propertyNames(t *testing.T) {
	t.Run("no properties", func(t *testing.T) {
		spec := NewObject()
		names := spec.propertyNames()
		assert.Len(t, names, 0)
	})
	t.Run("wrong field type", func(t *testing.T) {
		spec := NewObject()
		spec = append(spec, bson.E{Key: "properties", Value: "banana"})
		names := spec.propertyNames()
		assert.Len(t, names, 0)
		assert.Equal(t, cap(names), 0)
	})
	t.Run("a few fields", func(t *testing.T) {
		spec := NewObject()
		spec = append(spec, bson.E{Key: "properties", Value: bson.D{
			{Key: "name", Value: NewString()},
			{Key: "age", Value: NewInt()},
		}})
		names := spec.propertyNames()
		assert.Equal(t, []string{"name", "age"}, names)
	})
}
