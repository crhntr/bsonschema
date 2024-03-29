package bsonschema

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	doubleType    = "double"
	stringType    = "string"
	objectType    = "object"
	arrayType     = "array"
	objectIDType  = "objectId"
	dateType      = "date"
	boolType      = "bool"
	booleanType   = "boolean"
	nullType      = "null"
	regexType     = "regex"
	intType       = "int"
	timestampType = "timestamp"
	longType      = "long"
	decimalType   = "decimal"
	uuidType      = "uuid"
	binDataType   = "binData"

	//	numberType    = "number"
	// 	mixedType     = "mixed"

	bsonTypeField = "bsonType"

	allOfField       = "allOf"
	anyOfField       = "anyOf"
	notField         = "not"
	oneOfField       = "oneOf"
	enumField        = "enum"
	descriptionField = "description"

	exclusiveMaximumField = "exclusiveMaximum"
	exclusiveMinimumField = "exclusiveMinimum"
	maximumField          = "maximum"
	minimumField          = "minimum"
	multipleOfField       = "multipleOf"

	additionalPropertiesField = "additionalProperties"

	itemsField = "items"
)

type Specification interface {
	BSONDocument() bson.D
	BSONType() string
}

type Null bson.D

func NewNull() Null                            { return newSchema[Null]() }
func (spec Null) BSONDocument() bson.D         { return bson.D(spec) }
func (spec Null) BSONType() string             { return nullType }
func (spec Null) SetEnum(a bson.A) Null        { return setField(spec, enumField, a) }
func (spec Null) AllOf(a []Specification) Null { return setField(spec, allOfField, toA(a)) }
func (spec Null) AnyOf(a []Specification) Null { return setField(spec, anyOfField, toA(a)) }
func (spec Null) Not(a Specification) Null     { return setField(spec, notField, a.BSONDocument()) }
func (spec Null) OneOf(a []Specification) Null { return setField(spec, oneOfField, toA(a)) }
func (spec Null) SetDescription(d string) Null { return setField(spec, descriptionField, d) }

type Object bson.D

func NewObjectFromTypeWithTags(t reflect.Type) (Object, error) {
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected kind struct got %s", t.Kind())
	}
	zero := reflect.Zero(t).Interface()
	if _, ok := zero.(bson.Marshaler); ok {
		return nil, fmt.Errorf("expected type not to implement bson.Marshaler")
	}
	if _, ok := zero.(bson.ValueMarshaler); ok {
		return nil, fmt.Errorf("expected type not to implement bson.ValueUnmarshaler")
	}
	spec := NewObject()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name := propertyNameFromField(f)
		subSpec, err := newSpecFrom(f.Type)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", name, err)
		}
		spec = spec.AppendProperty(name, false, subSpec)
	}
	return spec, nil
}

func newSpecFrom(t reflect.Type) (Specification, error) {
	switch t.Kind() {
	case reflect.Struct:
		return NewObjectFromTypeWithTags(t)
	case reflect.String:
		return NewString(), nil
	case reflect.Float32, reflect.Float64:
		return NewDouble(), nil
	case reflect.Int, reflect.Int64:
		return NewLong(), nil
	case reflect.Int32, reflect.Int16, reflect.Int8:
		return NewInt(), nil
	case reflect.Bool:
		return NewBool(), nil
	case reflect.Slice, reflect.Array:
		subSpec, err := newSpecFrom(t.Elem())
		if err != nil {
			return nil, err
		}
		return NewArray().SetItems(subSpec), nil
	default:
		return nil, fmt.Errorf("unsupported kind %s", t.Kind())
	}
}

func NewObject() Object                               { return newSchema[Object]() }
func (spec Object) BSONDocument() bson.D              { return bson.D(spec) }
func (spec Object) BSONType() string                  { return objectType }
func (spec Object) SetEnum(a bson.A) Object           { return setField(spec, enumField, a) }
func (spec Object) AllOf(a []Specification) Object    { return setField(spec, allOfField, a) }
func (spec Object) AnyOf(a []Specification) Object    { return setField(spec, anyOfField, a) }
func (spec Object) Not(a Specification) Object        { return setField(spec, notField, a.BSONDocument()) }
func (spec Object) OneOf(a []Specification) Object    { return setField(spec, oneOfField, a) }
func (spec Object) SetDescription(d string) Object    { return setField(spec, descriptionField, d) }
func (spec Object) AppendRequired(name string) Object { return appendField(spec, "required", name) }
func (spec Object) SetTitle(value string) Object      { return setField(spec, "title", value) }
func (spec Object) AppendProperty(name string, required bool, specification Specification) Object {
	result := spec
	if required {
		result = result.AppendRequired(name)
	}
	for i, field := range spec {
		if field.Key == "properties" {
			result[i].Value = append(field.Value.(bson.D), bson.E{Key: name, Value: specification.BSONDocument()})
			return result
		}
	}
	result = setField(result, "properties", bson.D{
		{Key: name, Value: specification.BSONDocument()},
	})
	return result
}

func (spec Object) SetMinProperties(value int) Object  { return setField(spec, "minProperties", value) }
func (spec Object) SetMaxProperties(value bool) Object { return setField(spec, "maxProperties", value) }

func (spec Object) SetAdditionalProperties(value bool) Object {
	return setField(spec, additionalPropertiesField, value)
}

func (spec Object) AppendPatternProperty(expression string, specification Specification) Object {
	const field = "patternProperties"
	for _, el := range spec {
		if el.Key == field {
			el.Value = append(el.Value.(bson.D), bson.E{Key: expression, Value: specification.BSONDocument()})
			return spec
		}
	}
	return append(spec, bson.E{Key: field, Value: bson.D{{Key: expression, Value: specification.BSONDocument()}}})
}

func (spec Object) additionalProperties() bool {
	return !slices.ContainsFunc(spec, func(e bson.E) bool {
		val, _ := e.Value.(bool)
		return e.Key == additionalPropertiesField && !val
	})
}

func keysFromMarshalBSON(v reflect.Value) ([]string, error) {
	m, ok := v.Interface().(bson.Marshaler)
	if !ok {
		return nil, errors.New("expected type to implement bson.Marshaler")
	}
	buf, err := m.MarshalBSON()
	if err != nil {
		return nil, err
	}
	var d bson.D
	if err := bson.Unmarshal(buf, &d); err != nil {
		return nil, err
	}
	var keys []string
	for _, e := range d {
		keys = append(keys, e.Key)
	}
	return keys, nil
}

func bsonPropertyNamesFromStruct(t reflect.Type) ([]string, error) {
	if t.Implements(reflect.TypeOf((*bson.Marshaler)(nil)).Elem()) {
		return keysFromMarshalBSON(reflect.New(t))
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected kind struct got %s", t.Kind())
	}
	structTags := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		structTags = append(structTags, propertyNameFromField(f))
	}
	return structTags, nil
}

func propertyNameFromField(f reflect.StructField) string {
	tv, ok := f.Tag.Lookup("bson")
	if ok {
		tv, _, _ = strings.Cut(tv, ",")
		tv = strings.TrimSpace(tv)
	} else {
		tv = strings.ToLower(f.Name)
	}
	return tv
}

func (spec Object) propertyNames() []string {
	propertiesFieldIndex := slices.IndexFunc(spec, func(e bson.E) bool {
		return e.Key == "properties"
	})
	if propertiesFieldIndex < 0 {
		return nil
	}
	properties, ok := spec[propertiesFieldIndex].Value.(bson.D)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(properties))
	for _, name := range properties {
		names = append(names, name.Key)
	}
	return names
}

func (spec Object) CheckStructTags(t reflect.Type) error {
	structNames, err := bsonPropertyNamesFromStruct(t)
	if err != nil {
		return err
	}
	specNames := spec.propertyNames()
	for _, n := range structNames {
		if !slices.Contains(specNames, n) {
			return fmt.Errorf("missing specification property %q", n)
		}
	}
	for _, n := range specNames {
		if !slices.Contains(structNames, n) {
			return fmt.Errorf("missing struct field %q", n)
		}
	}
	return nil
}

type Array bson.D

func NewArray() Array                            { return newSchema[Array]() }
func (spec Array) BSONDocument() bson.D          { return bson.D(spec) }
func (spec Array) BSONType() string              { return arrayType }
func (spec Array) SetEnum(a bson.A) Array        { return setField(spec, enumField, a) }
func (spec Array) AllOf(a []Specification) Array { return setField(spec, allOfField, toA(a)) }
func (spec Array) AnyOf(a []Specification) Array { return setField(spec, anyOfField, toA(a)) }
func (spec Array) Not(a Specification) Array     { return setField(spec, notField, a.BSONDocument()) }
func (spec Array) OneOf(a []Specification) Array { return setField(spec, oneOfField, toA(a)) }
func (spec Array) SetDescription(d string) Array { return setField(spec, descriptionField, d) }

func (spec Array) SetItemsArray(items []Specification) Array {
	return setField(spec, itemsField, toA(items))
}

func (spec Array) SetItems(items Specification) Array {
	return setField(spec, itemsField, items.BSONDocument())
}
func (spec Array) SetMaxItems(value int) Array     { return setField(spec, "maxItems", value) }
func (spec Array) SetMinItems(value int) Array     { return setField(spec, "minItems", value) }
func (spec Array) SetUniqueItems(value bool) Array { return setField(spec, "uniqueItems", value) }
func (spec Array) SetAdditionalItems(value bool) Array {
	return setField(spec, "additionalItems", value)
}

func (spec Array) SetAdditionalItemsSchema(specification Specification) Array {
	return setField(spec, "additionalItems", specification.BSONDocument())
}
func (spec Array) SetMaxLength(value int) Array  { return setField(spec, "maxLength", value) }
func (spec Array) SetMinLength(value int) Array  { return setField(spec, "minLength", value) }
func (spec Array) SetPattern(value string) Array { return setField(spec, "pattern", value) }

type ObjectID bson.D

func NewObjectID() ObjectID                            { return newSchema[ObjectID]() }
func (spec ObjectID) BSONDocument() bson.D             { return bson.D(spec) }
func (spec ObjectID) BSONType() string                 { return objectIDType }
func (spec ObjectID) SetEnum(a bson.A) ObjectID        { return setField(spec, enumField, a) }
func (spec ObjectID) AllOf(a []Specification) ObjectID { return setField(spec, allOfField, toA(a)) }
func (spec ObjectID) AnyOf(a []Specification) ObjectID { return setField(spec, anyOfField, toA(a)) }
func (spec ObjectID) Not(a Specification) ObjectID {
	return setField(spec, notField, a.BSONDocument())
}
func (spec ObjectID) OneOf(a []Specification) ObjectID { return setField(spec, oneOfField, toA(a)) }
func (spec ObjectID) SetDescription(d string) ObjectID { return setField(spec, descriptionField, d) }

type String bson.D

func NewString() String                            { return newSchema[String]() }
func (spec String) BSONDocument() bson.D           { return bson.D(spec) }
func (spec String) BSONType() string               { return stringType }
func (spec String) SetEnum(a bson.A) String        { return setField(spec, enumField, a) }
func (spec String) AllOf(a []Specification) String { return setField(spec, allOfField, toA(a)) }
func (spec String) AnyOf(a []Specification) String { return setField(spec, anyOfField, toA(a)) }
func (spec String) Not(a Specification) String     { return setField(spec, notField, a.BSONDocument()) }
func (spec String) OneOf(a []Specification) String { return setField(spec, oneOfField, toA(a)) }
func (spec String) SetDescription(d string) String { return setField(spec, descriptionField, d) }
func (spec String) MinLength(d int) String         { return setField(spec, "minLength", d) }
func (spec String) MaxLength(d int) String         { return setField(spec, "maxLength", d) }

type Regex bson.D

func NewRegex() Regex                            { return newSchema[Regex]() }
func (spec Regex) BSONDocument() bson.D          { return bson.D(spec) }
func (spec Regex) BSONType() string              { return regexType }
func (spec Regex) SetEnum(a bson.A) Regex        { return setField(spec, enumField, a) }
func (spec Regex) AllOf(a []Specification) Regex { return setField(spec, allOfField, toA(a)) }
func (spec Regex) AnyOf(a []Specification) Regex { return setField(spec, anyOfField, toA(a)) }
func (spec Regex) Not(a Specification) Regex     { return setField(spec, notField, a.BSONDocument()) }
func (spec Regex) OneOf(a []Specification) Regex { return setField(spec, oneOfField, toA(a)) }
func (spec Regex) SetDescription(d string) Regex { return setField(spec, descriptionField, d) }

type UUID bson.D

func NewUUID() UUID                            { return newSchema[UUID]() }
func (spec UUID) BSONDocument() bson.D         { return bson.D(spec) }
func (spec UUID) BSONType() string             { return uuidType }
func (spec UUID) SetEnum(a bson.A) UUID        { return setField(spec, enumField, a) }
func (spec UUID) AllOf(a []Specification) UUID { return setField(spec, allOfField, toA(a)) }
func (spec UUID) AnyOf(a []Specification) UUID { return setField(spec, anyOfField, toA(a)) }
func (spec UUID) Not(a Specification) UUID     { return setField(spec, notField, a.BSONDocument()) }
func (spec UUID) OneOf(a []Specification) UUID { return setField(spec, oneOfField, toA(a)) }
func (spec UUID) SetDescription(d string) UUID { return setField(spec, descriptionField, d) }

type BinData bson.D

func NewBinData() BinData                            { return newSchema[BinData]() }
func (spec BinData) BSONDocument() bson.D            { return bson.D(spec) }
func (spec BinData) BSONType() string                { return binDataType }
func (spec BinData) SetEnum(a bson.A) BinData        { return setField(spec, enumField, a) }
func (spec BinData) AllOf(a []Specification) BinData { return setField(spec, allOfField, toA(a)) }
func (spec BinData) AnyOf(a []Specification) BinData { return setField(spec, anyOfField, toA(a)) }
func (spec BinData) Not(a Specification) BinData {
	return setField(spec, notField, a.BSONDocument())
}
func (spec BinData) OneOf(a []Specification) BinData { return setField(spec, oneOfField, toA(a)) }
func (spec BinData) SetDescription(d string) BinData { return setField(spec, descriptionField, d) }

type Timestamp bson.D

func NewTimestamp() Timestamp                     { return newSchema[Timestamp]() }
func (spec Timestamp) BSONDocument() bson.D       { return bson.D(spec) }
func (spec Timestamp) BSONType() string           { return timestampType }
func (spec Timestamp) SetEnum(a bson.A) Timestamp { return setField(spec, enumField, a) }
func (spec Timestamp) AllOf(a []Specification) Timestamp {
	return setField(spec, allOfField, toA(a))
}

func (spec Timestamp) AnyOf(a []Specification) Timestamp {
	return setField(spec, anyOfField, toA(a))
}

func (spec Timestamp) Not(a Specification) Timestamp {
	return setField(spec, notField, a.BSONDocument())
}

func (spec Timestamp) OneOf(a []Specification) Timestamp {
	return setField(spec, oneOfField, toA(a))
}
func (spec Timestamp) SetDescription(d string) Timestamp { return setField(spec, descriptionField, d) }

type Date bson.D

func NewDate() Date                            { return newSchema[Date]() }
func (spec Date) BSONDocument() bson.D         { return bson.D(spec) }
func (spec Date) BSONType() string             { return dateType }
func (spec Date) SetEnum(a bson.A) Date        { return setField(spec, enumField, a) }
func (spec Date) AllOf(a []Specification) Date { return setField(spec, allOfField, toA(a)) }
func (spec Date) AnyOf(a []Specification) Date { return setField(spec, anyOfField, toA(a)) }
func (spec Date) Not(a Specification) Date     { return setField(spec, notField, a.BSONDocument()) }
func (spec Date) OneOf(a []Specification) Date { return setField(spec, oneOfField, toA(a)) }
func (spec Date) SetDescription(d string) Date { return setField(spec, descriptionField, d) }

type Bool bson.D

func NewBool() Bool                            { return newSchema[Bool]() }
func (spec Bool) BSONDocument() bson.D         { return bson.D(spec) }
func (spec Bool) BSONType() string             { return boolType }
func (spec Bool) SetEnum(a bson.A) Bool        { return setField(spec, enumField, a) }
func (spec Bool) AllOf(a []Specification) Bool { return setField(spec, allOfField, toA(a)) }
func (spec Bool) AnyOf(a []Specification) Bool { return setField(spec, anyOfField, toA(a)) }
func (spec Bool) Not(a Specification) Bool     { return setField(spec, notField, a.BSONDocument()) }
func (spec Bool) OneOf(a []Specification) Bool { return setField(spec, oneOfField, toA(a)) }
func (spec Bool) SetDescription(d string) Bool { return setField(spec, descriptionField, d) }

type Boolean bson.D

func NewBoolean() Boolean                            { return newSchema[Boolean]() }
func (spec Boolean) BSONDocument() bson.D            { return bson.D(spec) }
func (spec Boolean) BSONType() string                { return booleanType }
func (spec Boolean) SetEnum(a bson.A) Boolean        { return setField(spec, enumField, a) }
func (spec Boolean) AllOf(a []Specification) Boolean { return setField(spec, allOfField, toA(a)) }
func (spec Boolean) AnyOf(a []Specification) Boolean { return setField(spec, anyOfField, toA(a)) }
func (spec Boolean) Not(a Specification) Boolean {
	return setField(spec, notField, a.BSONDocument())
}
func (spec Boolean) OneOf(a []Specification) Boolean { return setField(spec, oneOfField, toA(a)) }
func (spec Boolean) SetDescription(d string) Boolean { return setField(spec, descriptionField, d) }

type Double bson.D

func NewDouble() Double                            { return newSchema[Double]() }
func (spec Double) BSONDocument() bson.D           { return bson.D(spec) }
func (spec Double) BSONType() string               { return doubleType }
func (spec Double) SetEnum(a bson.A) Double        { return setField(spec, enumField, a) }
func (spec Double) AllOf(a []Specification) Double { return setField(spec, allOfField, toA(a)) }
func (spec Double) AnyOf(a []Specification) Double { return setField(spec, anyOfField, toA(a)) }
func (spec Double) Not(a Specification) Double     { return setField(spec, notField, a.BSONDocument()) }
func (spec Double) OneOf(a []Specification) Double { return setField(spec, oneOfField, toA(a)) }
func (spec Double) SetDescription(d string) Double { return setField(spec, descriptionField, d) }
func (spec Double) SetExclusiveMaximum(n float64) Double {
	return setField(spec, exclusiveMaximumField, n)
}

func (spec Double) SetExclusiveMinimum(n float64) Double {
	return setField(spec, exclusiveMinimumField, n)
}
func (spec Double) SetMaximum(n float64) Double    { return setField(spec, maximumField, n) }
func (spec Double) SetMinimum(n float64) Double    { return setField(spec, minimumField, n) }
func (spec Double) SetMultipleOf(n float64) Double { return setField(spec, multipleOfField, n) }

type Int bson.D

func NewInt() Int                                { return newSchema[Int]() }
func (spec Int) BSONDocument() bson.D            { return bson.D(spec) }
func (spec Int) BSONType() string                { return intType }
func (spec Int) SetEnum(a bson.A) Int            { return setField(spec, enumField, a) }
func (spec Int) AllOf(a []Specification) Int     { return setField(spec, allOfField, toA(a)) }
func (spec Int) AnyOf(a []Specification) Int     { return setField(spec, anyOfField, toA(a)) }
func (spec Int) Not(a Specification) Int         { return setField(spec, notField, a.BSONDocument()) }
func (spec Int) OneOf(a []Specification) Int     { return setField(spec, oneOfField, toA(a)) }
func (spec Int) SetDescription(d string) Int     { return setField(spec, descriptionField, d) }
func (spec Int) SetExclusiveMaximum(n int32) Int { return setField(spec, exclusiveMaximumField, n) }
func (spec Int) SetExclusiveMinimum(n int32) Int { return setField(spec, exclusiveMinimumField, n) }
func (spec Int) SetMaximum(n int32) Int          { return setField(spec, maximumField, n) }
func (spec Int) SetMinimum(n int32) Int          { return setField(spec, minimumField, n) }
func (spec Int) SetMultipleOf(n int32) Int       { return setField(spec, multipleOfField, n) }

type Long bson.D

func NewLong() Long                                { return newSchema[Long]() }
func (spec Long) BSONDocument() bson.D             { return bson.D(spec) }
func (spec Long) BSONType() string                 { return longType }
func (spec Long) SetEnum(a bson.A) Long            { return setField(spec, enumField, a) }
func (spec Long) AllOf(a []Specification) Long     { return setField(spec, allOfField, toA(a)) }
func (spec Long) AnyOf(a []Specification) Long     { return setField(spec, anyOfField, toA(a)) }
func (spec Long) Not(a Specification) Long         { return setField(spec, notField, a.BSONDocument()) }
func (spec Long) OneOf(a []Specification) Long     { return setField(spec, oneOfField, toA(a)) }
func (spec Long) SetDescription(d string) Long     { return setField(spec, descriptionField, d) }
func (spec Long) SetExclusiveMaximum(n int64) Long { return setField(spec, exclusiveMaximumField, n) }
func (spec Long) SetExclusiveMinimum(n int64) Long { return setField(spec, exclusiveMinimumField, n) }
func (spec Long) SetMaximum(n int64) Long          { return setField(spec, maximumField, n) }
func (spec Long) SetMinimum(n int64) Long          { return setField(spec, minimumField, n) }
func (spec Long) SetMultipleOf(n int64) Long       { return setField(spec, multipleOfField, n) }

type Decimal bson.D

func NewDecimal() Decimal                            { return newSchema[Decimal]() }
func (spec Decimal) BSONDocument() bson.D            { return bson.D(spec) }
func (spec Decimal) BSONType() string                { return decimalType }
func (spec Decimal) SetEnum(a bson.A) Decimal        { return setField(spec, enumField, a) }
func (spec Decimal) AllOf(a []Specification) Decimal { return setField(spec, allOfField, toA(a)) }
func (spec Decimal) AnyOf(a []Specification) Decimal { return setField(spec, anyOfField, toA(a)) }
func (spec Decimal) Not(a Specification) Decimal {
	return setField(spec, notField, a.BSONDocument())
}
func (spec Decimal) OneOf(a []Specification) Decimal { return setField(spec, oneOfField, toA(a)) }
func (spec Decimal) SetDescription(d string) Decimal { return setField(spec, descriptionField, d) }
func (spec Decimal) SetExclusiveMaximum(n primitive.Decimal128) Decimal {
	return setField(spec, exclusiveMaximumField, n)
}

func (spec Decimal) SetExclusiveMinimum(n primitive.Decimal128) Decimal {
	return setField(spec, exclusiveMinimumField, n)
}

func (spec Decimal) SetMaximum(n primitive.Decimal128) Decimal {
	return setField(spec, maximumField, n)
}

func (spec Decimal) SetMinimum(n primitive.Decimal128) Decimal {
	return setField(spec, minimumField, n)
}

func (spec Decimal) SetMultipleOf(n primitive.Decimal128) Decimal {
	return setField(spec, multipleOfField, n)
}

type sharedInterface[T Specification] interface {
	~[]bson.E

	Specification

	SetDescription(value string) T
	SetEnum(values bson.A) T
	AllOf(array []Specification) T
	AnyOf(array []Specification) T
	Not(array Specification) T
	OneOf(array []Specification) T
}

func newSchema[T sharedInterface[T]]() T {
	return T(bson.D{{Key: bsonTypeField, Value: T(nil).BSONType()}})
}

func setField[D ~[]bson.E, T any](doc D, field string, value T) D {
	for _, el := range doc {
		if el.Key == field {
			el.Value = value
			return doc
		}
	}
	return append(doc, bson.E{
		Key: field, Value: value,
	})
}

func appendField[D ~[]bson.E, T any](doc D, field string, value T) D {
	for i, el := range doc {
		if el.Key == field {
			switch existing := el.Value.(type) {
			case []T:
				doc[i].Value = append(existing, value)
				return doc
			case bson.A:
				doc[i].Value = append(existing, value)
				return doc
			default:
				doc[i].Value = append(bson.A{existing}, value)
				return doc
			}
		}
	}
	return append(doc, bson.E{
		Key: field, Value: bson.A{value},
	})
}

func toA[T interface{ BSONDocument() bson.D }](slice []T) bson.A {
	result := make(bson.A, 0, len(slice))
	for _, s := range slice {
		result = append(result, s.BSONDocument())
	}
	return result
}
