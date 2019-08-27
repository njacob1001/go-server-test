package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-chi/chi"
	_ "github.com/lib/pq"
	"github.com/njacob1001/truora/models"
)

// TimestampLayout constant
const TimestampLayout = "2006-01-02 15:04:05"

// Env persisting database
type Env struct {
	db models.Datastore
}

// SslLabsEnpoint format of each enpoint of SslLabsResponse
type SslLabsEnpoint struct {
	IPAddress         string `json:"ipAddress"`
	ServerName        string `json:"serverName"`
	StatusMessage     string `json:"StatusMessage"`
	Grade             string `json:"grade"`
	GradeTrustIgnored string `json:"gradeTrustIgnored"`
	IsExceptional     bool   `json:"isExceptional"`
	Progress          int8   `json:"progress"`
	Duration          int32  `json:"duration"`
	Delegation        int8   `json:"delegation"`
}

// SslLabsResponse format of SSLLabs API
type SslLabsResponse struct {
	Host      string           `json:"host"`
	Port      int              `json:"port"`
	Protocol  string           `json:"protocol"`
	IsPublic  bool             `json:"isPublic"`
	Status    string           `json:"status"`
	Message   string           `json:"statusMessage"`
	Endpoints []SslLabsEnpoint `json:"endpoints"`
}

// WhoIsIPResponse struct
type WhoIsIPResponse struct {
	As      string `json:"as"`
	Country string `json:"country"`
}

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
}

func (env *Env) getConsultedServers(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	type Response struct {
		Ok      bool     `json:"ok"`
		Message bool     `json:"message"`
		Domains []string `json:"domains"`
	}

	domains, err := env.db.AllDomains()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	domainsString := []string{}
	for _, domain := range domains {
		domainsString = append(domainsString, domain.Domain)
	}
	resp := Response{
		Ok:      true,
		Domains: domainsString,
	}
	js, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

// serverHandler calls to sslabs api
func (env *Env) serverHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	domainName := chi.URLParam(r, "serverName")

	oneHourAgo := time.Now().Add(-60 * time.Minute).Format(TimestampLayout)
	allowCheckChanges := false
	// Get domain information from SSLLabs API
	hostInfo, err := http.Get("https://api.ssllabs.com/api/v3/analyze?host=" + domainName)
	if err != nil {
		log.Fatalln(err)
	}
	defer hostInfo.Body.Close()

	var hostResponse SslLabsResponse
	if err := json.NewDecoder(hostInfo.Body).Decode(&hostResponse); err != nil {
		log.Println(err)
	}

	type ErrorMessage struct {
		Ok      bool   `json:"ok"`
		Status  string `json:"status"`
		IsDown  bool   `json:"is_down"`
		Message string `json:"message"`
	}
	if strings.EqualFold(hostResponse.Status, "error") {
		resp := ErrorMessage{
			Ok:      false,
			Status:  hostResponse.Status,
			IsDown:  true,
			Message: "ERROR",
		}
		js, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
		return
	}

	if !strings.EqualFold(hostResponse.Status, "READY") {
		resp := ErrorMessage{
			Ok:      false,
			Status:  hostResponse.Status,
			Message: hostResponse.Message,
		}
		js, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
		return
	}
	var servers []models.Server
	for _, server := range hostResponse.Endpoints {

		serverInfo, err := http.Get("http://ip-api.com/json/" + server.IPAddress)
		if err != nil {
			log.Fatalln(err)
		}
		defer serverInfo.Body.Close()
		var details WhoIsIPResponse
		if err := json.NewDecoder(serverInfo.Body).Decode(&details); err != nil {
			log.Println(err)
		}
		servers = append(servers, models.Server{
			Address:  server.IPAddress,
			SSLGrade: server.Grade,
			Country:  details.Country,
			Owner:    details.As,
		})
	}
	// Get HomePage
	homePage, err := http.Get("http://" + domainName)
	if err != nil {
		log.Println(err)
	}
	defer homePage.Body.Close()
	doc, err := goquery.NewDocumentFromReader(homePage.Body)
	htmlTitle := ""
	htmlIcon := ""
	doc.Find("head").Each(func(i int, s *goquery.Selection) {
		htmlTitle = s.Find("title").Text()
	})
	doc.Find("link").Each(func(i int, s *goquery.Selection) {
		if name, _ := s.Attr("type"); strings.EqualFold(name, "image/x-icon") {
			htmlIcon, _ = s.Attr("href")
		}
	})
	if htmlIcon == "" {
		fmt.Println("FINDING OTHER ICON")
		doc.Find("link").Each(func(i int, s *goquery.Selection) {
			if name, _ := s.Attr("rel"); strings.EqualFold(name, "shortcut icon") {
				htmlIcon, _ = s.Attr("href")
			}
		})
	}

	sort.Slice(servers, func(i, j int) bool {
		return servers[i].SSLGrade < servers[j].SSLGrade
	})
	domainFormat := models.DomainInfo{
		Domain:           domainName,
		ServersChanged:   false,
		SSLGrade:         servers[len(servers)-1].SSLGrade,
		PreviousSSLGrade: "",
		Logo:             htmlIcon,
		Title:            htmlTitle,
		IsDown:           false,
		LastUpdated:      time.Now().Format(TimestampLayout),
		Servers:          servers,
	}

	domain, err := env.db.GetDomainInformation(domainName)
	domainFormat.PreviousSSLGrade = domain.SSLGrade

	if err != nil && err == sql.ErrNoRows {
		err := env.db.InsertDomain(&domainFormat)
		if err != nil {
			log.Println(err)
		}
		resp := models.DomainInfo{
			Domain:           domainFormat.Domain,
			LastUpdated:      domainFormat.LastUpdated,
			ServersChanged:   false,
			SSLGrade:         domainFormat.SSLGrade,
			PreviousSSLGrade: domainFormat.PreviousSSLGrade,
			Logo:             domainFormat.Logo,
			Title:            domainFormat.Title,
			IsDown:           false,
			Servers:          domainFormat.Servers,
		}
		js, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
		fmt.Println("CREATE NEW DOMAIN")
		return
	} else if err != nil {
		resp := ErrorMessage{
			Ok:      false,
			Status:  "ERROR",
			IsDown:  true,
			Message: "ERROR",
		}
		js, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
		return
	}
	formatedDate, err := time.Parse("2006-01-02T15:04:05Z", domain.LastUpdated)
	ee, _ := time.Parse(TimestampLayout, oneHourAgo)
	if formatedDate.Before(ee) {
		allowCheckChanges = true
	}
	serversDB, err := env.db.GetServers(domain.Domain)

	if err != nil && err != sql.ErrNoRows {
		log.Println(err)
	}
	for _, serverDB := range serversDB {
		domain.Servers = append(domain.Servers, *serverDB)
	}
	changed := false
	if allowCheckChanges {
		//  Check if are changes to delete
		for _, serverDB := range serversDB {
			foundIP := false
			for _, newServer := range domainFormat.Servers {
				if serverDB.Address == newServer.Address {
					foundIP = true
					if serverDB.Country != newServer.Country || serverDB.Owner != newServer.Country || serverDB.SSLGrade != newServer.SSLGrade {
						err := env.db.UpdateServer(&newServer)
						if err != nil {
							panic(err)
						}
						if !changed {
							changed = true
						}
					}
				}
			}
			if !foundIP {
				fmt.Printf("Deleting ip %s...\n", serverDB.Address)
				err := env.db.DeleteServer(serverDB.Address)
				if err != nil {
					panic(err)
				}
				if !changed {
					changed = true
				}
			}
			foundIP = false
		}
		// check if there are new IPs to insert
		for _, newServer := range domainFormat.Servers {
			foundNewIP := true
			for _, serverDB := range serversDB {
				if newServer.Address == serverDB.Address {
					foundNewIP = false
				}
			}
			if foundNewIP {
				err := env.db.InsertServer(&newServer, domainFormat.Domain)
				if err != nil {
					panic(err)
				}
				if !changed {
					changed = true
				}
			}
			foundNewIP = true
		}
		if changed || domain.SSLGrade != domainFormat.SSLGrade || domain.Logo != domainFormat.Logo || domain.Title != domainFormat.Title {
			err := env.db.UpdateDomain(&domainFormat)
			if err != nil {
				panic(err)
			}
		}

		resp := models.DomainInfo{
			ServersChanged:   changed,
			Domain:           domainFormat.Domain,
			LastUpdated:      domainFormat.LastUpdated,
			SSLGrade:         domainFormat.SSLGrade,
			PreviousSSLGrade: domain.SSLGrade,
			Logo:             domainFormat.Logo,
			Title:            domainFormat.Title,
			IsDown:           false,
			Servers:          domainFormat.Servers,
		}
		js, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
		fmt.Println("DETECTED CHANGES IN DATABASE")
		return
	}

	fmt.Println("IGNORE CHANGES IN DATABASE")
	resp := models.DomainInfo{
		Domain:           domain.Domain,
		LastUpdated:      domain.LastUpdated,
		ServersChanged:   false,
		SSLGrade:         domain.SSLGrade,
		PreviousSSLGrade: domain.PreviousSSLGrade,
		Logo:             domain.Logo,
		Title:            domain.Title,
		IsDown:           false,
		Servers:          domain.Servers,
	}
	js, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func main() {

	db, err := models.NewDB("postgresql://jacob@localhost:26257/truora?ssl=false&sslmode=disable&password=worker")

	if err != nil {
		panic(err)
	}

	env := &Env{db: db}
	// defer db.Close()

	r := chi.NewRouter()

	r.Get("/api/domain/{serverName}", env.serverHandler)
	r.Get("/api/consulted", env.getConsultedServers)

	s := &http.Server{
		Addr:           ":3500",
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	s.ListenAndServe()
}
