package bsonschema_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/crhntr/bsonschema"
)

func TestBSONSpecification_New(t *testing.T) {
	types := []bsonschema.Specification{
		bsonschema.NewNull(),
		bsonschema.NewObject(),
		bsonschema.NewArray(),
		bsonschema.NewObjectID(),
		bsonschema.NewString(),
		bsonschema.NewRegex(),
		bsonschema.NewUUID(),
		bsonschema.NewBinData(),
		bsonschema.NewTimestamp(),
		bsonschema.NewDate(),
		bsonschema.NewBool(),
		bsonschema.NewBoolean(),
		bsonschema.NewDouble(),
		bsonschema.NewInt(),
		bsonschema.NewLong(),
		bsonschema.NewDecimal(),
	}

	for _, bt := range types {
		t.Run(bt.BSONType(), func(t *testing.T) {
			v := reflect.ValueOf(bt)
			if typeName := strings.ToLower(v.Type().Name()); typeName != strings.ToLower(bt.BSONType()) {
				t.Errorf("got %v want %v", typeName, strings.ToLower(bt.BSONType()))
			}
			document := bt.BSONDocument()
			if len(document) != 1 {
				t.Fatalf("got %v want %v", len(document), 1)
			}
			bsonTypeElement := document[0]
			if bsonTypeElement.Key != "bsonType" {
				t.Errorf("got %v want %v", document[0].Key, "bsonType")
			}
			if bsonTypeElement.Value.(string) != bt.BSONType() {
				t.Errorf("got %v want %v", bsonTypeElement.Value, bt.BSONType())
			}
		})
	}
}

func TestObject_CheckStructTags(t *testing.T) {
	t.Run("missing struct field", func(t *testing.T) {
		err := bsonschema.NewObject().
			AppendProperty("description", false, bsonschema.NewString()).
			AppendProperty("name", false, bsonschema.NewString()).
			CheckStructTags(reflect.TypeOf(struct {
				Description string `json:"bson:description"`
			}{}))
		assert.ErrorContains(t, err, `missing struct field "name"`)
	})
	t.Run("missing spec field", func(t *testing.T) {
		err := bsonschema.NewObject().
			AppendProperty("description", false, bsonschema.NewString()).
			CheckStructTags(reflect.TypeOf(struct {
				Description string `json:"bson:description"`
				Name        string `json:"bson:name"`
			}{}))
		assert.ErrorContains(t, err, `missing specification property "name"`)
	})
	t.Run("properties and fields do not have the same order", func(t *testing.T) {
		err := bsonschema.NewObject().
			AppendProperty("name", false, bsonschema.NewString()).
			AppendProperty("description", false, bsonschema.NewString()).
			CheckStructTags(reflect.TypeOf(struct {
				Description string `json:"bson:description"`
				Name        string `json:"bson:name"`
			}{}))
		assert.NoError(t, err)
	})
	t.Run("wrong type for parameter", func(t *testing.T) {
		err := bsonschema.NewObject().CheckStructTags(reflect.TypeOf(100))
		assert.ErrorContains(t, err, "expected kind struct got int")
	})
}
