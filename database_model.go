package goapp

import (
	"errors"
	"log"
	"reflect"
	"time"

	"github.com/mitoteam/mttools"
	gorm "gorm.io/gorm"
)

// base model type all model types should embed
type DbModel struct {
}

// gorm.Model alternative without DeletedAt column (to disable Soft Delete feature)
// see https://gorm.io/docs/delete.html#Soft-Delete
type BaseModel struct {
	DbModel

	ID        int64 `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// gorm TX object. Prepared in PreQuery(), used in LoadObject, LoadOL, CountOL
var gormTx *gorm.DB

// Prepares gorm TX for loading model (O)bjects (L)ist.
// Returned TX can be used to apply conditions and other gorm query clauses.
func PreQuery[ModelT any]() (tx *gorm.DB) {
	var modelObject ModelT

	if !checkSchemaModelType(reflect.TypeOf(modelObject)) {
		return nil
	}

	gormTx = DbSchema.Db().Model(&modelObject)
	return gormTx
}

// Loads model object by ID. Returns nil if object was not loaded.
func LoadO[ModelT any](id any) (r *ModelT) {
	defer func() { gormTx = nil }()

	typedId, ok := mttools.AnyToInt64Ok(id)

	if !ok || typedId == 0 { //id is empty
		return nil
	}

	if gormTx == nil { // gormTx is not prepared
		if gormTx = PreQuery[ModelT](); gormTx == nil { //unable to prepare
			return nil
		}
	}

	var modelObject ModelT

	if err := gormTx.First(&modelObject, typedId).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Println("Query ERROR: " + err.Error())
		}

		return nil
	}

	return &modelObject
}

// Works like LoadO() but panics if object was not found.
func LoadOMust[ModelT any](id any) (r *ModelT) {
	r = LoadO[ModelT](id)

	if r == nil {
		log.Panicf("Can not load model object %s[ID=%v]", reflect.TypeFor[ModelT]().String(), id)
	}

	return r
}

// If id == 0 creates new empty object. Loads model object by ID otherwise.
func LoadOrCreateO[ModelT any](id any) (r *ModelT) {
	typedId, ok := mttools.AnyToInt64Ok(id)

	if !ok {
		return nil // id type is unknown
	}

	if typedId == 0 {
		return new(ModelT)
	} else {
		return LoadO[ModelT](typedId)
	}
}

// Loads first available model object. Conditions can be set in PreQuery().
// Returns nil if object was not loaded.
func FirstO[ModelT any]() (r *ModelT) {
	defer func() { gormTx = nil }()

	if gormTx == nil { // gormTx is not prepared
		if gormTx = PreQuery[ModelT](); gormTx == nil { //unable to prepare
			return nil
		}
	}

	var modelObject ModelT

	if err := gormTx.First(&modelObject).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Println("Query ERROR: " + err.Error())
		}

		return nil
	}

	return &modelObject
}

// Deletes object. Returns false if something goes wrong.
func DeleteObject(modelObject any) error {
	var t reflect.Type

	if t, modelObject = modelObjectReflection(modelObject); t == nil {
		return errors.New("modelObject is not valid schema object")
	}

	//make sure it is saved object (with ID set)
	v := reflect.ValueOf(modelObject).Elem() // struct itself from pointer
	if v.FieldByName(reflect.TypeFor[BaseModel]().Name()).FieldByName("ID").Int() == 0 {
		return errors.New("modelObject has ID=0")
	}

	if err := DbSchema.Db().Delete(modelObject).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		} else {
			log.Println("Query ERROR: " + err.Error())
			return err
		}
	}

	return nil
}

// Saves object. Returns false if something goes wrong.
func SaveObject(modelObject any) bool {
	var t reflect.Type

	if t, modelObject = modelObjectReflection(modelObject); t == nil {
		// not a schema model object
		return false
	}

	if err := DbSchema.Db().Save(modelObject).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Println("Query ERROR: " + err.Error())
		}

		return false
	}

	return true
}

// Inserts new record for object. Used for objects without ID to force creation.
// Normal objects should use SaveObject() for both inserting and updating.
// Returns false if something goes wrong.
func CreateObject(modelObject any) bool {
	var t reflect.Type

	if t, modelObject = modelObjectReflection(modelObject); t == nil {
		// not a schema model object
		return false
	}

	if err := DbSchema.Db().Create(modelObject).Error; err != nil {
		log.Println("Query ERROR: " + err.Error())
		return false
	}

	return true
}

// Loads model (O)bjects (L)ist using prepared gorm TX - PreQuery().
// if gorm TX was not prepared, empty one is created (selecting all model objects)
func LoadOL[ModelT any]() (list []*ModelT) {
	list = []*ModelT{} //empty list by default
	defer func() { gormTx = nil }()

	if gormTx == nil {
		if gormTx = PreQuery[ModelT](); gormTx == nil {
			return
		}
	}

	if err := gormTx.Find(&list).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Println("Query ERROR: " + err.Error())
		}
	}

	return list
}

// Counts records for model (O)bjects using prepared gorm TX - PreQuery().
// if gorm TX was not prepared, empty one is created (counting all model objects)
func CountOL[ModelT any]() (cnt int64) {
	cnt = 0
	defer func() { gormTx = nil }()

	if gormTx == nil {
		if gormTx = PreQuery[ModelT](); gormTx == nil {
			return
		}
	}

	if err := gormTx.Count(&cnt).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Println("Query ERROR: " + err.Error())
		}
	}

	return cnt
}

// Just a simple wrapper
func Transaction(trxF func() error) {
	DbSchema.Db().Transaction(func(tx *gorm.DB) error {
		return trxF()
	})
}

// Checks if t is type of registered model struct
func checkSchemaModelType(t reflect.Type) bool {
	if DbSchema.Db() == nil {
		//database is not opened
		return false
	}

	if !DbSchema.HasModel(t) {
		log.Printf("ERROR[checkSchemaModelType]: unknown model '%s'\n", t.String())
		return false
	}

	return true
}

// returns t  = Type of model structure, o = pointer to model object
func modelObjectReflection(modelObject any) (t reflect.Type, o any) {
	t = reflect.TypeOf(modelObject)

	//check if it is a model object pointer and dereference it's type
	if t.Kind() == reflect.Pointer {
		t = t.Elem()    // t is pointer, dereference it's type to struct
		o = modelObject // already a pointer
	} else if t.Kind() == reflect.Struct {
		o = reflect.ValueOf(modelObject).Addr().Interface() // modelObject is struct, return pointer to it
	}

	if !checkSchemaModelType(t) {
		return nil, nil
	}

	return t, o
}
