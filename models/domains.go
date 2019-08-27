package models

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// Server format for each server
type Server struct {
	Address  string `json:"address"`
	SSLGrade string `json:"ssl_grade"`
	Country  string `json:"country"`
	Owner    string `json:"owner"`
}

// DomainInfo format for each Domain
type DomainInfo struct {
	Domain           string `json:"domain"`
	ServersChanged   bool   `json:"servers_changed"`
	SSLGrade         string `json:"ssl_grade"`
	PreviousSSLGrade string `json:"previous_ssl_grade"`
	Logo             string `json:"logo"`
	Title            string `json:"title"`
	IsDown           bool   `json:"is_down"`
	LastUpdated      string `json:"last_updated"`
	Servers          []Server
}

// AllDomains get all domains
func (db *DB) AllDomains() ([]*DomainInfo, error) {
	rows, err := db.Query("SELECT domain FROM domains")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	domains := make([]*DomainInfo, 0)
	for rows.Next() {
		domain := new(DomainInfo)
		err := rows.Scan(&domain.Domain)
		if err != nil {
			return nil, err
		}
		domains = append(domains, domain)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return domains, nil
}

// GetDomainInformation function
func (db *DB) GetDomainInformation(domainName string) (*DomainInfo, error) {
	query := fmt.Sprintf("SELECT * FROM domains WHERE domain='%s'", domainName)
	row := db.QueryRow(query)
	domain := new(DomainInfo)
	err := row.Scan(&domain.Domain, &domain.ServersChanged, &domain.SSLGrade, &domain.PreviousSSLGrade, &domain.Logo, &domain.Title, &domain.IsDown, &domain.LastUpdated)
	switch err {
	case sql.ErrNoRows:
		return domain, err
	case nil:
		return domain, nil
	default:
		panic(err)
	}
}

// UpsertDomain function
func (db *DB) UpsertDomain(domain *DomainInfo) error {
	//TODO: complete this function
	query := fmt.Sprintf(`
		INSERT INTO domains (
			domain, servers_changed, ssl_grade, previous_ssl_grade, logo, title, is_down
		) VALUES (
			'%s', %t, '%s', '%s', '%s', '%s', %t
		)
		ON CONFLICT (domain) DO UPDATE SET servers_changed=%[2]t, ssl_grade='%[3]%s' previous_ssl_grade='%[4]s', logo='%[5]s', title='%[6]s', is_down='%[7]t'
	`, domain.Domain, domain.ServersChanged, domain.SSLGrade, domain.PreviousSSLGrade, domain.Logo, domain.Title, domain.IsDown)
	_, err := db.Exec(query)
	if err != nil {
		return err
	}
	return nil
}

// UpserServer function
func (db *DB) UpserServer(server *Server, fkDomain string) error {
	_, err := db.Exec(`
		INSERT INTO servers (
			address, ssl_grade, country, owner, fk_domain
		) VALUES (
			$1, $2, $3, $4, $5
		) ON CONFLICT (address) DO UPDATE SET
		ssl_grade=$2, country=$3, owner=$4
	`, server.Address, server.SSLGrade, server.Country, server.Owner, fkDomain)
	if err != nil {
		return err
	}
	return nil
}

// UpdateServer function
func (db *DB) UpdateServer(server *Server) error {
	_, err := db.Exec("UPDATE servers SET ssl_grade=$1, country=$2, owner=$3 WHERE address=$4", server.SSLGrade, server.Country, server.Owner, server.Address)
	if err != nil {
		return err
	}
	return nil
}

// InsertServer function
func (db *DB) InsertServer(server *Server, fk string) error {
	_, err := db.Exec(`INSERT INTO servers (
		address, ssl_grade, country, owner, fk_domain
		) VALUES (
			$1, $2, $3, $4, $5
		)`, server.Address, server.SSLGrade, server.Country, server.Owner, fk)
	if err != nil {
		return err
	}
	return nil

}

// DeleteServer function
func (db *DB) DeleteServer(address string) error {
	_, err := db.Exec("DELETE from servers WHERE address=$1", address)
	if err != nil {
		return err
	}
	return nil
}

// InsertDomain function
func (db *DB) InsertDomain(domain *DomainInfo) error {
	_, err := db.Exec(`
		INSERT INTO domains (
			domain, servers_changed, ssl_grade, previous_ssl_grade, logo, title, is_down, last_updated
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
	`, domain.Domain, domain.ServersChanged, domain.SSLGrade, domain.PreviousSSLGrade, domain.Logo, domain.Title, domain.IsDown, domain.LastUpdated)
	if err != nil {
		log.Println(err)
	}
	for _, server := range domain.Servers {
		_, err := db.Exec(`
			INSERT INTO servers (
				address, ssl_grade, country, owner, fk_domain
			) VALUES (
				$1, $2, $3, $4, $5
			)
		`, server.Address, server.SSLGrade, server.Country, server.Owner, domain.Domain)
		if err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}
	return nil
}

// GetServers function
func (db *DB) GetServers(domain string) ([]*Server, error) {
	query := fmt.Sprintf("SELECT * FROM servers WHERE fk_domain='%s'", domain)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	servers := make([]*Server, 0)
	for rows.Next() {
		server := new(Server)
		fk := ""
		err := rows.Scan(&server.Address, &server.SSLGrade, &server.Country, &server.Owner, &fk)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return servers, nil
}

// UpdateDate function
func (db *DB) UpdateDate(domain string) error {
	_, err := db.Exec("UPDATE domains SET last_updated=$1 WHERE domain=$2", time.Now(), domain)
	if err != nil {
		return err
	}
	return nil
}

// UpdateDomain function
func (db *DB) UpdateDomain(domain *DomainInfo) error {
	_, err := db.Exec(`
		UPDATE domains SET 
		servers_changed=$1,
		ssl_grade=$2,
		previous_ssl_grade=$3,
		logo=$4,
		title=$5,
		is_down=$6,
		last_updated=$7
		WHERE domain=$8
	`, true, domain.SSLGrade, domain.PreviousSSLGrade, domain.Logo, domain.Title, domain.IsDown, domain.LastUpdated, domain.Domain)
	if err != nil {
		return err
	}
	return nil
}
