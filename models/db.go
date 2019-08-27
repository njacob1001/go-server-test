package models

import (
	"database/sql"
	"log"
	"time"
)

// Datastore interface
type Datastore interface {
	AllDomains() ([]*DomainInfo, error)
	GetDomainInformation(domanName string) (*DomainInfo, error)
	UpsertDomain(domain *DomainInfo) error
	UpdateDate(domain string) error
	UpserServer(server *Server, fkDomain string) error
	GetServers(domain string) ([]*Server, error)
	InsertDomain(domain *DomainInfo) error
	UpdateServer(server *Server) error
	DeleteServer(address string) error
	InsertServer(server *Server, fk string) error
	UpdateDomain(domain *DomainInfo) error
}

// DB database
type DB struct {
	*sql.DB
}

// NewDB create a connection with a postgressdatabase
func NewDB(dataSourceName string) (*DB, error) {
	initializeDomains := `
		CREATE TABLE IF NOT EXISTS 
		domains (
			domain VARCHAR (50) PRIMARY KEY,
			servers_changed BOOLEAN,
			ssl_grade VARCHAR (5),
			previous_ssl_grade VARCHAR (5),
			logo VARCHAR (150),
			title VARCHAR (50),
			is_down BOOLEAN,
			last_updated timestamp
		)`
	initializeServers := `
		CREATE TABLE IF NOT EXISTS
		servers (
			address VARCHAR (50) PRIMARY KEY,
			ssl_grade VARCHAR (5),
			country VARCHAR (50),
			owner VARCHAR (50),
			fk_domain VARCHAR (50) NOT NULL REFERENCES domains(domain)
		)
	`
	defaultValue := `
		INSERT INTO 
		domains 
		(domain, servers_changed, ssl_grade, previous_ssl_grade, logo, title, is_down, last_updated) 
		VALUES 
		('testing.com', false, 'A', 'B', 'http://imgur.com/logo.png','testing title', false, $1)
		ON CONFLICT DO NOTHING
	`
	db, err := sql.Open("postgres", dataSourceName)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		return nil, err
	}
	if _, err := db.Exec(initializeDomains); err != nil {
		log.Fatal(err)
		return nil, err
	}
	if _, err := db.Exec(initializeServers); err != nil {
		log.Fatal(err)
		return nil, err
	}
	if _, err := db.Exec(defaultValue, time.Now()); err != nil {
		log.Fatal(err)
		return nil, err
	}
	return &DB{db}, nil
}
