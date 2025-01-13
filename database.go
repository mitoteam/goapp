package goapp

import (
	"log"
	"os"
	"reflect"
	"time"

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

	//ensure it embeds DbModel
	if !mttools.IsStructTypeEmbeds(modelType, reflect.TypeFor[DbModel]()) {
		log.Panicf("modelType %s does not embed DbModel", modelType.String())
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

	gormLogger := logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
		SlowThreshold:             500 * time.Millisecond,
		IgnoreRecordNotFoundError: true,
		Colorful:                  true,
	})

	if logSql {
		gormLogger.LogMode(logger.Info)
	}

	config.Logger = gormLogger

	db_schema.db, err = gorm.Open(sqlite.Open(dbFileName), config)

	if err != nil {
		return err
	}

	log.Printf("Database %s opened\n", dbFileName)

	// Migrate the schema
	for name, modelObject := range db_schema.modelMap {
		if err := db_schema.db.AutoMigrate(modelObject); err != nil {
			log.Panicf("ERROR migrating %s: %s\n", name, err.Error())
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
