package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/thda/tds"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Configuración de la conexión a la base de datos
type Config struct {
	DBType   string
	Server   string
	Port     int
	User     string
	Password string
	Database string
	Schema   string
	Output   string
	SSLMode  string // Para PostgreSQL
}

// Estructura para almacenar la información de una columna
type Column struct {
	ColumnName   string `json:"columnName"`
	DataType     string `json:"dataType"`
	IsNullable   string `json:"isNullable"`
	MaxLength    int    `json:"maxLength,omitempty"`
	Precision    int    `json:"precision,omitempty"`
	Scale        int    `json:"scale,omitempty"`
	IsPrimaryKey bool   `json:"isPrimaryKey"`
	IsIdentity   bool   `json:"isIdentity"`
	DefaultValue string `json:"defaultValue,omitempty"`
}

// Estructura para almacenar la información de una tabla
type Table struct {
	TableName string   `json:"tableName"`
	Schema    string   `json:"schema"`
	Columns   []Column `json:"columns"`
}

// Estructura principal que contiene todas las tablas
type DatabaseSchema struct {
	DatabaseName string  `json:"databaseName"`
	DBType       string  `json:"dbType"`
	Schema       string  `json:"defaultSchema"`
	Tables       []Table `json:"tables"`
}

// Estructura para MongoDB
type MongoCollection struct {
	CollectionName string                 `json:"collectionName"`
	DatabaseName   string                 `json:"databaseName"`
	Indexes        []MongoIndex           `json:"indexes,omitempty"`
	SampleDocument map[string]interface{} `json:"sampleDocument,omitempty"`
}

type MongoIndex struct {
	Name   string          `json:"name"`
	Keys   []MongoIndexKey `json:"keys"`
	Unique bool            `json:"unique"`
}

type MongoIndexKey struct {
	Field     string `json:"field"`
	Direction int    `json:"direction"`
}

type MongoSchema struct {
	DatabaseName string            `json:"databaseName"`
	DBType       string            `json:"dbType"`
	Collections  []MongoCollection `json:"collections"`
}

func main() {
	// Definir flags
	dbType := flag.String("dbtype", "", "Tipo de base de datos (sqlserver, sybase, mysql, postgres, mongodb)")
	server := flag.String("server", "localhost", "Servidor de la base de datos")
	port := flag.Int("port", 0, "Puerto de la base de datos (se usará el puerto por defecto según el tipo)")
	user := flag.String("user", "", "Usuario de la base de datos")
	password := flag.String("password", "", "Contraseña de la base de datos")
	database := flag.String("database", "", "Nombre de la base de datos")
	schema := flag.String("schema", "dbo", "Schema por defecto (para bases de datos que lo soportan)")
	output := flag.String("output", "database_schema.json", "Archivo de salida JSON")
	sslMode := flag.String("sslmode", "disable", "Modo SSL (para PostgreSQL)")
	help := flag.Bool("help", false, "Mostrar ayuda")

	flag.Parse()

	// Mostrar ayuda si se solicita
	if *help {
		printHelp()
		return
	}

	// Validar parámetros requeridos
	if *dbType == "" || *user == "" || *password == "" || *database == "" {
		fmt.Println("Error: Los parámetros dbtype, user, password y database son requeridos")
		fmt.Println("\nUso:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Configurar puerto por defecto según el tipo de BD
	if *port == 0 {
		*port = getDefaultPort(*dbType)
	}

	// Configuración de la conexión
	config := Config{
		DBType:   strings.ToLower(*dbType),
		Server:   *server,
		Port:     *port,
		User:     *user,
		Password: *password,
		Database: *database,
		Schema:   *schema,
		Output:   *output,
		SSLMode:  *sslMode,
	}

	// Validar tipo de base de datos
	if !isValidDBType(config.DBType) {
		fmt.Printf("Error: Tipo de base de datos no válido: %s\n", config.DBType)
		fmt.Println("Tipos válidos: sqlserver, sybase, mysql, postgres, mongodb")
		os.Exit(1)
	}

	fmt.Printf("Configuración:\n")
	fmt.Printf("  Tipo de BD: %s\n", config.DBType)
	fmt.Printf("  Servidor: %s:%d\n", config.Server, config.Port)
	fmt.Printf("  Base de datos: %s\n", config.Database)
	fmt.Printf("  Schema: %s\n", config.Schema)
	fmt.Printf("  Archivo de salida: %s\n", config.Output)
	fmt.Println()

	// Procesar según el tipo de base de datos
	if config.DBType == "mongodb" {
		processMongoDB(config)
	} else {
		processSQLDatabase(config)
	}
}

func isValidDBType(dbType string) bool {
	validTypes := []string{"sqlserver", "sybase", "mysql", "postgres", "mongodb"}
	for _, t := range validTypes {
		if dbType == t {
			return true
		}
	}
	return false
}

func getDefaultPort(dbType string) int {
	switch dbType {
	case "sqlserver":
		return 1433
	case "sybase":
		return 5000
	case "mysql":
		return 3306
	case "postgres":
		return 5432
	case "mongodb":
		return 27017
	default:
		return 0
	}
}

func processSQLDatabase(config Config) {
	// Crear cadena de conexión según el tipo de BD
	connectionString := getConnectionString(config)

	// Determinar el driver según el tipo de BD
	driverName := getDriverName(config.DBType)

	// Conectar a la base de datos
	db, err := sql.Open(driverName, connectionString)
	if err != nil {
		log.Fatal("Error al conectar a la base de datos:", err)
	}
	defer db.Close()

	// Verificar la conexión
	err = db.Ping()
	if err != nil {
		log.Fatal("Error al verificar la conexión:", err)
	}

	fmt.Printf("✅ Conexión exitosa a %s\n", strings.ToUpper(config.DBType))

	// Extraer el esquema de la base de datos
	schema, err := extractDatabaseSchema(db, config)
	if err != nil {
		log.Fatal("Error al extraer el esquema:", err)
	}

	// Guardar en archivo JSON
	err = saveToJSONFile(schema, config.Output)
	if err != nil {
		log.Fatal("Error al guardar el archivo JSON:", err)
	}

	fmt.Printf("✅ Esquema guardado en: %s\n", config.Output)
	fmt.Printf("📊 Total de tablas procesadas: %d\n", len(schema.Tables))
}

func processMongoDB(config Config) {
	// Crear cadena de conexión para MongoDB
	connectionString := fmt.Sprintf("mongodb://%s:%s@%s:%d/%s",
		config.User, config.Password, config.Server, config.Port, config.Database)

	client, err := mongo.Connect(nil, options.Client().ApplyURI(connectionString))
	if err != nil {
		log.Fatal("Error al conectar a MongoDB:", err)
	}
	defer client.Disconnect(nil)

	// Verificar la conexión
	err = client.Ping(nil, nil)
	if err != nil {
		log.Fatal("Error al verificar la conexión a MongoDB:", err)
	}

	fmt.Printf("✅ Conexión exitosa a MongoDB\n")

	// Extraer el esquema de MongoDB
	schema, err := extractMongoDBSchema(client, config.Database)
	if err != nil {
		log.Fatal("Error al extraer el esquema de MongoDB:", err)
	}

	// Guardar en archivo JSON
	err = saveToJSONFile(schema, config.Output)
	if err != nil {
		log.Fatal("Error al guardar el archivo JSON:", err)
	}

	fmt.Printf("✅ Esquema de MongoDB guardado en: %s\n", config.Output)
	fmt.Printf("📊 Total de colecciones procesadas: %d\n", len(schema.Collections))
}

func getDriverName(dbType string) string {
	switch dbType {
	case "sqlserver":
		return "sqlserver"
	case "sybase":
		return "tds"
	case "mysql":
		return "mysql"
	case "postgres":
		return "postgres"
	default:
		return ""
	}
}

func getConnectionString(config Config) string {
	switch config.DBType {
	case "sqlserver":
		return fmt.Sprintf("server=%s;port=%d;user id=%s;password=%s;database=%s",
			config.Server, config.Port, config.User, config.Password, config.Database)
	case "sybase":
		return fmt.Sprintf("tds://%s:%s@%s:%d/%s?charset=utf8",
			config.User, config.Password, config.Server, config.Port, config.Database)
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
			config.User, config.Password, config.Server, config.Port, config.Database)
	case "postgres":
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			config.Server, config.Port, config.User, config.Password, config.Database, config.SSLMode)
	default:
		return ""
	}
}

func extractDatabaseSchema(db *sql.DB, config Config) (*DatabaseSchema, error) {
	schema := &DatabaseSchema{
		DatabaseName: config.Database,
		DBType:       config.DBType,
		Schema:       config.Schema,
		Tables:       []Table{},
	}

	// Consulta para obtener tablas según el tipo de BD
	queryTables := getTablesQuery(config.DBType, config.Schema)

	rowsTables, err := db.Query(queryTables)
	if err != nil {
		return nil, fmt.Errorf("error al consultar tablas: %v", err)
	}
	defer rowsTables.Close()

	fmt.Printf("🔍 Extrayendo información de tablas...\n")

	for rowsTables.Next() {
		var tableSchema, tableName string

		// Manejar diferentes estructuras de resultados según la BD
		switch config.DBType {
		case "sqlserver", "sybase":
			err = rowsTables.Scan(&tableSchema, &tableName)
		case "mysql":
			err = rowsTables.Scan(&tableSchema, &tableName)
		case "postgres":
			err = rowsTables.Scan(&tableSchema, &tableName)
		}

		if err != nil {
			return nil, fmt.Errorf("error al escanear tabla: %v", err)
		}

		// Obtener columnas para esta tabla
		columns, err := extractTableColumns(db, config.DBType, tableSchema, tableName)
		if err != nil {
			return nil, fmt.Errorf("error al extraer columnas para tabla %s: %v", tableName, err)
		}

		table := Table{
			TableName: tableName,
			Schema:    tableSchema,
			Columns:   columns,
		}

		schema.Tables = append(schema.Tables, table)
		fmt.Printf("  📋 Tabla procesada: %s.%s (%d columnas)\n", tableSchema, tableName, len(columns))
	}

	if err = rowsTables.Err(); err != nil {
		return nil, fmt.Errorf("error iterando sobre tablas: %v", err)
	}

	return schema, nil
}

func getTablesQuery(dbType string, defaultSchema string) string {
	switch dbType {
	case "sqlserver":
		return fmt.Sprintf(`
			SELECT 
				TABLE_SCHEMA,
				TABLE_NAME
			FROM INFORMATION_SCHEMA.TABLES
			WHERE TABLE_TYPE = 'BASE TABLE'
			AND TABLE_SCHEMA = '%s'
			ORDER BY TABLE_SCHEMA, TABLE_NAME
		`, defaultSchema)
	case "sybase":
		// Consulta simplificada para Sybase - obtener todas las tablas del usuario/schema
		return fmt.Sprintf(`
			SELECT 
				user_name(uid) as schema_name,
				name as table_name
			FROM sysobjects 
			WHERE type = 'U'  -- Tablas de usuario
			AND user_name(uid) = '%s'
			ORDER BY schema_name, table_name
		`, defaultSchema)
	case "mysql":
		return `
			SELECT 
				TABLE_SCHEMA,
				TABLE_NAME
			FROM INFORMATION_SCHEMA.TABLES
			WHERE TABLE_TYPE = 'BASE TABLE'
			AND TABLE_SCHEMA = DATABASE()
			ORDER BY TABLE_SCHEMA, TABLE_NAME
		`
	case "postgres":
		return fmt.Sprintf(`
			SELECT 
				table_schema,
				table_name
			FROM information_schema.tables
			WHERE table_type = 'BASE TABLE'
			AND table_schema = '%s'
			ORDER BY table_schema, table_name
		`, defaultSchema)
	default:
		return ""
	}
}

func extractTableColumns(db *sql.DB, dbType, schemaName, tableName string) ([]Column, error) {
	// Para Sybase, construimos la consulta dinámicamente sin parámetros
	if dbType == "sybase" {
		return extractSybaseTableColumns(db, tableName)
	}

	queryColumns := getColumnsQuery(dbType)
	var rowsColumns *sql.Rows
	var err error

	// Usar parámetros preparados correctamente para cada base de datos
	switch dbType {
	case "sqlserver":
		rowsColumns, err = db.Query(queryColumns, sql.Named("schema", schemaName), sql.Named("table", tableName))
	case "mysql":
		rowsColumns, err = db.Query(queryColumns, schemaName, tableName)
	case "postgres":
		// PostgreSQL usa $1, $2 para parámetros
		rowsColumns, err = db.Query(queryColumns, schemaName, tableName)
	default:
		return nil, fmt.Errorf("tipo de base de datos no soportado: %s", dbType)
	}

	if err != nil {
		return nil, fmt.Errorf("error al consultar columnas: %v", err)
	}
	defer rowsColumns.Close()

	var columns []Column

	for rowsColumns.Next() {
		col, err := scanColumn(rowsColumns, dbType)
		if err != nil {
			return nil, err
		}
		columns = append(columns, col)
	}

	if err = rowsColumns.Err(); err != nil {
		return nil, fmt.Errorf("error iterando sobre columnas: %v", err)
	}

	return columns, nil
}

// Función específica para extraer columnas de Sybase (sin parámetros)
func extractSybaseTableColumns(db *sql.DB, tableName string) ([]Column, error) {
	// Consulta simplificada para Sybase - sin la parte compleja de claves primarias que causa errores
	query := fmt.Sprintf(`
		SELECT 
			c.name as column_name,
			t.name as data_type,
			c.length,
			c.prec as numeric_precision,
			c.scale as numeric_scale,
			CASE 
				WHEN c.status & 8 = 8 THEN 'YES' 
				ELSE 'NO' 
			END as is_nullable,
			CASE 
				WHEN c.status & 128 = 128 THEN 1 
				ELSE 0 
			END as is_identity,
			ISNULL(OBJECT_NAME(c.cdefault), '') as default_value,
			0 as is_primary_key  -- Por ahora, no detectamos claves primarias para evitar errores
		FROM syscolumns c
		JOIN systypes t ON c.usertype = t.usertype
		WHERE c.id = object_id('%s')
		ORDER BY c.colid
	`, tableName)

	rowsColumns, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error al consultar columnas: %v", err)
	}
	defer rowsColumns.Close()

	var columns []Column

	for rowsColumns.Next() {
		var col Column
		var isNullable string
		var length, prec, scale sql.NullInt32
		var isPrimaryKey, isIdentity int

		err := rowsColumns.Scan(
			&col.ColumnName,
			&col.DataType,
			&length,
			&prec,
			&scale,
			&isNullable,
			&isIdentity,
			&col.DefaultValue,
			&isPrimaryKey,
		)
		if err != nil {
			return nil, fmt.Errorf("error al escanear columna: %v", err)
		}

		// Convertir valores
		col.IsNullable = isNullable
		col.IsPrimaryKey = (isPrimaryKey == 1)
		col.IsIdentity = (isIdentity == 1)

		if length.Valid {
			col.MaxLength = int(length.Int32)
		}
		if prec.Valid {
			col.Precision = int(prec.Int32)
		}
		if scale.Valid {
			col.Scale = int(scale.Int32)
		}

		columns = append(columns, col)
	}

	if err = rowsColumns.Err(); err != nil {
		return nil, fmt.Errorf("error iterando sobre columnas: %v", err)
	}

	// Intentar obtener información de claves primarias por separado
	primaryKeys, err := getSybasePrimaryKeys(db, tableName)
	if err != nil {
		// Si hay error, simplemente continuamos sin información de PKs
		fmt.Printf("  ⚠️  No se pudieron obtener claves primarias para %s: %v\n", tableName, err)
	} else {
		// Actualizar las columnas que son claves primarias
		for i, col := range columns {
			if _, isPK := primaryKeys[col.ColumnName]; isPK {
				columns[i].IsPrimaryKey = true
			}
		}
	}

	return columns, nil
}

// Función separada para obtener claves primarias en Sybase
func getSybasePrimaryKeys(db *sql.DB, tableName string) (map[string]bool, error) {
	primaryKeys := make(map[string]bool)

	// Consulta alternativa para obtener claves primarias en Sybase
	query := fmt.Sprintf(`
		SELECT 
			sc.name as column_name
		FROM sysindexes i
		JOIN syscolumns sc ON i.id = sc.id AND sc.colid IN (i.key1, i.key2, i.key3, i.key4, i.key5, i.key6, i.key7, i.key8)
		JOIN sysobjects o ON i.id = o.id
		WHERE o.name = '%s'
		AND i.status & 2 = 2  -- Índice único
		AND EXISTS (
			SELECT 1 
			FROM sysconstraints ct 
			WHERE ct.tableid = i.id 
			AND ct.constrid = i.indid 
			AND ct.status & 1 = 1  -- Clave primaria
		)
	`, tableName)

	rows, err := db.Query(query)
	if err != nil {
		// Si esta consulta falla, intentamos una más simple
		return getSybasePrimaryKeysSimple(db, tableName)
	}
	defer rows.Close()

	for rows.Next() {
		var columnName string
		err := rows.Scan(&columnName)
		if err != nil {
			return nil, err
		}
		primaryKeys[columnName] = true
	}

	return primaryKeys, nil
}

// Consulta alternativa más simple para claves primarias
func getSybasePrimaryKeysSimple(db *sql.DB, tableName string) (map[string]bool, error) {
	primaryKeys := make(map[string]bool)

	query := fmt.Sprintf(`
		SELECT 
			col_name(i.id, k.keyno) as column_name
		FROM sysindexes i, syskeys k
		WHERE i.id = object_id('%s')
		AND i.id = k.id
		AND i.indid = k.indid
		AND i.status & 2 = 2  -- Índice único
		AND EXISTS (
			SELECT 1 
			FROM sysconstraints ct 
			WHERE ct.tableid = i.id 
			AND ct.constrid = i.indid 
			AND ct.status & 1 = 1  -- Clave primaria
		)
	`, tableName)

	rows, err := db.Query(query)
	if err != nil {
		// Si también falla, retornamos mapa vacío
		return primaryKeys, nil
	}
	defer rows.Close()

	for rows.Next() {
		var columnName string
		err := rows.Scan(&columnName)
		if err != nil {
			return nil, err
		}
		primaryKeys[columnName] = true
	}

	return primaryKeys, nil
}

func getColumnsQuery(dbType string) string {
	switch dbType {
	case "sqlserver":
		return `
			SELECT 
				c.COLUMN_NAME,
				c.DATA_TYPE,
				c.IS_NULLABLE,
				c.CHARACTER_MAXIMUM_LENGTH,
				c.NUMERIC_PRECISION,
				c.NUMERIC_SCALE,
				CASE WHEN pk.COLUMN_NAME IS NOT NULL THEN 1 ELSE 0 END AS IS_PRIMARY_KEY,
				COLUMNPROPERTY(OBJECT_ID(c.TABLE_SCHEMA + '.' + c.TABLE_NAME), c.COLUMN_NAME, 'IsIdentity') AS IS_IDENTITY,
				COALESCE(c.COLUMN_DEFAULT, '') AS COLUMN_DEFAULT
			FROM INFORMATION_SCHEMA.COLUMNS c
			LEFT JOIN (
				SELECT 
					ku.TABLE_SCHEMA,
					ku.TABLE_NAME,
					ku.COLUMN_NAME
				FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
				INNER JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE ku
					ON tc.CONSTRAINT_TYPE = 'PRIMARY KEY'
					AND tc.CONSTRAINT_NAME = ku.CONSTRAINT_NAME
			) pk ON c.TABLE_SCHEMA = pk.TABLE_SCHEMA 
				AND c.TABLE_NAME = pk.TABLE_NAME 
				AND c.COLUMN_NAME = pk.COLUMN_NAME
			WHERE c.TABLE_SCHEMA = @schema 
				AND c.TABLE_NAME = @table
			ORDER BY c.ORDINAL_POSITION
		`
	case "mysql":
		return `
			SELECT 
				COLUMN_NAME,
				DATA_TYPE,
				IS_NULLABLE,
				CHARACTER_MAXIMUM_LENGTH,
				NUMERIC_PRECISION,
				NUMERIC_SCALE,
				CASE WHEN COLUMN_KEY = 'PRI' THEN 1 ELSE 0 END AS IS_PRIMARY_KEY,
				CASE WHEN EXTRA LIKE '%auto_increment%' THEN 1 ELSE 0 END AS IS_IDENTITY,
				COALESCE(COLUMN_DEFAULT, '') AS COLUMN_DEFAULT
			FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
			ORDER BY ORDINAL_POSITION
		`
	case "postgres":
		return `
			SELECT 
				column_name,
				data_type,
				is_nullable,
				character_maximum_length,
				numeric_precision,
				numeric_scale,
				CASE 
					WHEN (SELECT COUNT(*) 
						  FROM information_schema.key_column_usage k
						  JOIN information_schema.table_constraints tc 
						  ON k.constraint_name = tc.constraint_name 
						  AND k.table_schema = tc.table_schema
						  WHERE k.table_schema = $1 
							AND k.table_name = $2 
							AND k.column_name = c.column_name
							AND tc.constraint_type = 'PRIMARY KEY') > 0 
					THEN 1 
					ELSE 0 
				END AS is_primary_key,
				CASE 
					WHEN column_default LIKE 'nextval%' THEN 1 
					ELSE 0 
				END AS is_identity,
				COALESCE(column_default, '') AS column_default
			FROM information_schema.columns c
			WHERE table_schema = $1 
			  AND table_name = $2
			ORDER BY ordinal_position
		`
	default:
		return ""
	}
}

func scanColumn(rows *sql.Rows, dbType string) (Column, error) {
	var col Column
	var isNullable string
	var charMaxLength, numericPrecision, numericScale sql.NullInt32
	var isPrimaryKey, isIdentity int

	switch dbType {
	case "sqlserver":
		err := rows.Scan(
			&col.ColumnName,
			&col.DataType,
			&isNullable,
			&charMaxLength,
			&numericPrecision,
			&numericScale,
			&isPrimaryKey,
			&isIdentity,
			&col.DefaultValue,
		)
		if err != nil {
			return col, err
		}
	case "mysql", "postgres":
		err := rows.Scan(
			&col.ColumnName,
			&col.DataType,
			&isNullable,
			&charMaxLength,
			&numericPrecision,
			&numericScale,
			&isPrimaryKey,
			&isIdentity,
			&col.DefaultValue,
		)
		if err != nil {
			return col, err
		}
	}

	// Convertir valores comunes
	col.IsNullable = isNullable
	col.IsPrimaryKey = (isPrimaryKey == 1)
	col.IsIdentity = (isIdentity == 1)

	if charMaxLength.Valid {
		col.MaxLength = int(charMaxLength.Int32)
	}
	if numericPrecision.Valid {
		col.Precision = int(numericPrecision.Int32)
	}
	if numericScale.Valid {
		col.Scale = int(numericScale.Int32)
	}

	return col, nil
}

func extractMongoDBSchema(client *mongo.Client, databaseName string) (*MongoSchema, error) {
	schema := &MongoSchema{
		DatabaseName: databaseName,
		DBType:       "mongodb",
		Collections:  []MongoCollection{},
	}

	// Obtener lista de colecciones
	collections, err := client.Database(databaseName).ListCollectionNames(nil, nil)
	if err != nil {
		return nil, err
	}

	fmt.Printf("🔍 Extrayendo información de colecciones...\n")

	for _, collName := range collections {
		fmt.Printf("  📁 Procesando colección: %s\n", collName)

		collection := MongoCollection{
			CollectionName: collName,
			DatabaseName:   databaseName,
			Indexes:        []MongoIndex{},
		}

		// Aquí podrías agregar lógica para extraer índices y documentos de muestra
		// Por simplicidad, solo agregamos la colección básica

		schema.Collections = append(schema.Collections, collection)
	}

	return schema, nil
}

func saveToJSONFile(data interface{}, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error al crear archivo: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	err = encoder.Encode(data)
	if err != nil {
		return fmt.Errorf("error al codificar JSON: %v", err)
	}

	return nil
}

func printHelp() {
	fmt.Println("🚀 Extractor de Esquema de Base de Datos Multiplataforma")
	fmt.Println("========================================================")
	fmt.Println("Este programa extrae la estructura de bases de datos SQL y NoSQL")
	fmt.Println("y las guarda en un archivo JSON.")
	fmt.Println()
	fmt.Println("📋 Parámetros:")
	fmt.Println("  -dbtype    Tipo de base de datos (sqlserver, sybase, mysql, postgres, mongodb) *REQUERIDO*")
	fmt.Println("  -server    Servidor de la base de datos (default: localhost)")
	fmt.Println("  -port      Puerto de la base de datos (default: según el tipo de BD)")
	fmt.Println("  -user      Usuario de la base de datos *REQUERIDO*")
	fmt.Println("  -password  Contraseña de la base de datos *REQUERIDO*")
	fmt.Println("  -database  Nombre de la base de datos *REQUERIDO*")
	fmt.Println("  -schema    Schema por defecto (default: dbo)")
	fmt.Println("  -output    Archivo de salida JSON (default: database_schema.json)")
	fmt.Println("  -sslmode   Modo SSL para PostgreSQL (default: disable)")
	fmt.Println("  -help      Mostrar esta ayuda")
	fmt.Println()
	fmt.Println("💡 Ejemplos de uso:")
	fmt.Println("  SQL Server: ./extractor -dbtype sqlserver -user sa -password secret -database MiDB -schema dbo -output esquema.json")
	fmt.Println("  PostgreSQL: ./extractor -dbtype postgres -user postgres -password pass -database MiDB -schema public -output esquema.json")
	fmt.Println("  MySQL:      ./extractor -dbtype mysql -user root -password pass -database MiDB -output esquema.json")
	fmt.Println("  Sybase:     ./extractor -dbtype sybase -user sa -password secret -database MiDB -schema dbo -output esquema.json")
	fmt.Println("  MongoDB:    ./extractor -dbtype mongodb -user admin -password pass -database MiDB -output esquema.json")
	fmt.Println("  Ayuda:      ./extractor -help")
	fmt.Println()
	fmt.Println("🔧 Valores por defecto:")
	fmt.Println("  SQL Server: puerto 1433, schema dbo")
	fmt.Println("  Sybase:     puerto 5000, schema dbo")
	fmt.Println("  MySQL:      puerto 3306, schema nombre_de_la_base")
	fmt.Println("  PostgreSQL: puerto 5432, schema public")
	fmt.Println("  MongoDB:    puerto 27017")
}
