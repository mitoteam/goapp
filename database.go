package goapp

import (
	"log"
	"reflect"

	"github.com/glebarez/sqlite"
	"github.com/mitoteam/mttools"
	gorm "gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

const dbFileName = "data.db"

type dbSchemaType struct {
	modelMap map[string]any // name = typename, value = empty struct of this type
	db       *gorm.DB
}

var DbSchema *dbSchemaType

func init() {
	DbSchema = &dbSchemaType{}

	DbSchema.modelMap = make(map[string]any, 0) //typeName => modelObject
}

func (schema *dbSchemaType) AddModel(modelType reflect.Type) {
	//ensure it is a struct
	if modelType.Kind() != reflect.Struct {
		log.Panicf("modelType %s is not a struct", modelType.String())
	}

	//ensure it embeds BaseModel
	if !mttools.IsStructTypeEmbeds(modelType, reflect.TypeFor[BaseModel]()) {
		log.Panicf("modelType %s does not embed BaseModel", modelType.String())
	}

	//crate empty model object
	schema.modelMap[modelType.String()] = reflect.New(modelType).Elem().Interface()
}

func (schema *dbSchemaType) HasModel(modelType reflect.Type) bool {
	_, exists := schema.modelMap[modelType.String()]
	return exists
}

func (schema *dbSchemaType) Db() *gorm.DB {
	return schema.db
}

func (db_schema *dbSchemaType) Open(logSql bool) error {
	var err error

	config := &gorm.Config{
		//Logger: logger.Default.LogMode(logger.Warn),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, // use singular table name, table for `User` would be `user` with this option enabled
		},
	}

	if logSql {
		config.Logger = logger.Default.LogMode(logger.Info)
	}

	db_schema.db, err = gorm.Open(sqlite.Open(dbFileName), config)

	if err != nil {
		return err
	}

	log.Printf("Database %s opened\n", dbFileName)

	// Migrate the schema
	//log.Printf("DBG: %+v\n", db_schema.modelMap)
	for name, modelObject := range db_schema.modelMap {
		//log.Printf("DBG: %s %+v\n", name, modelObject)
		if err := db_schema.db.AutoMigrate(modelObject); err != nil {
			log.Printf("ERROR migrating %s: %s", name, err.Error())
		}
	}

	log.Printf("Database migration done (schema model count: %d)\n", len(db_schema.modelMap))

	return nil
}

func (schema *dbSchemaType) Close() {
	sqlDB, err := schema.db.DB()

	if err != nil {
		sqlDB.Close()
	}

	log.Printf("Database %s closed\n", dbFileName)

	schema.db = nil
}
