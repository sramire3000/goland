# Compilar
go build -o extractor

# SQL Server con schema dbo (por defecto)
./extractor -dbtype sqlserver -user sa -password "Password123" -database Arreconsa -schema dbo -output arreconsa_esquema.json

# MySQL (no usa schema en el mismo sentido, pero mantiene compatibilidad)
./extractor -dbtype mysql -user root -password "password" -database zipkin -output zipkin_esquema.json


# PostgreSQL con schema public
./extractor -dbtype postgres -user postgres -password "password" -database companies -schema public -output company_esquema.json


# Sybase con schema dbo (por defecto)
./extractor -dbtype sybase -user sa -password "password" -database test -schema dbo -output test_esquema.json


# Ayuda completa
./extractor -help