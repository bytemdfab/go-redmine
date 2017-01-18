/*

Package redmine provides an API for interacting with a Redmine server.

Note that this is a read-only API. There is not currently any support for
updating information in Redmine.

*/
package redmine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

var client = &http.Client{}

// structures ///////////////////////////

// Session represents an active connection to a Redmine server.
type Session struct {
	username string
	password string
	url      string
	apiKey   string
}

// User represents a Redmine user.
type User struct {
	Id          int    `json:"id"`
	ApiKey      string `json:"api_key"`
	Login       string `json:"login"`
	FirstName   string `json:"firstname"`
	LastName    string `json:"lastname"`
	Mail        string `json:"mail"`
	LastLoginOn string `json:"last_login_on"`
}

// Group represents a Redmine group
type Group struct {
	Name         string `json:"name,omitempty"`
	Id           int    `json:"id,omitempty"`
	UserIds      []int  `json:"user_ids"`
	CustomFields []ValueField `json:"custom_fields,omitempty"`
}

// Project represents a Redmine project.
type Project struct {
	CreatedOn   string `json:"created_on"`
	Description string `json:"description"`
	Id          int    `json:"id"`
	IsPublic    bool   `json:"is_public"`
	Name        string `json:"name"`
	UpdatedOn   string `json:"updated_on"`
}

// Group represents a single issue in Redmine.
type Issue struct {
	AssignedTo     Identifier   `json:"assigned_to,omitempty"`
	Author         Identifier   `json:"author,omitempty"`
	Category       Identifier   `json:"category,omitempty"`
	CreatedOn      string       `json:"created_on,omitempty"`
	CustomFields   []ValueField `json:"custom_fields,omitempty"`
	Description    string       `json:"description,omitempty"`
	DoneRatio      int          `json:"done_ratio,omitempty"`
	DueDate        string       `json:"due_date,omitempty"`
	EstimatedHours float64      `json:"estimated_hours,omitempty"`
	Id             int          `json:"id,omitempty"`
	Priority       Identifier   `json:"priority,omitempty"`
	Project        Identifier   `json:"project,omitempty"`
	StartDate      string       `json:"start_date,omitempty"`
	Status         IssueStatus  `json:"status,omitempty"`
	Subject        string       `json:"subject,omitempty"`
	Tracker        Identifier   `json:"tracker,omitempty"`
	UpdatedOn      string       `json:"updated_on,omitempty"`
}

// UpdateIssue is used to pass updates to Redmine.
type UpdateIssue struct {
	AssignedTo     int     `json:"assigned_to_id,omitempty"`
	Author         int     `json:"author_id,omitempty"`
	Category       int     `json:"category_id,omitempty"`
	CreatedOn      string  `json:"created_on,omitempty"`
	Description    string  `json:"description,omitempty"`
	DoneRatio      int     `json:"done_ratio,omitempty"`
	DueDate        string  `json:"due_date,omitempty"`
	EstimatedHours float64 `json:"estimated_hours,omitempty"`
	Priority       int     `json:"priority_id,omitempty"`
	Project        int     `json:"project_id,omitempty"`
	StartDate      string  `json:"start_date,omitempty"`
	Status         int     `json:"status_id,omitempty"`
	Subject        string  `json:"subject,omitempty"`
	Tracker        int     `json:"tracker_id,omitempty"`
	UpdatedOn      string  `json:"updated_on,omitempty"`
}

// IssueStatus represents one of the issue statuses configured in Redmine.
type IssueStatus struct {
	Id        int    `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	IsDefault bool   `json:"is_default,omitempty"`
	IsClosed  bool   `json:"is_closed,omitempty"`
}

// TimeEntry represents a single time entry.
type TimeEntry struct {
	Id        int        `json:"id"`
	Hours     float64    `json:"hours"`
	CreatedOn string     `json:"created_on"`
	SpentOn   string     `json:"spent_on"`
	UpdatedOn string     `json:"updated_on"`
	User      Identifier `json:"user"`
	Project   Identifier `json:"project"`
	Activity  Identifier `json:"activity"`
	Issue     struct {
		Id int `json:"id"`
	} `json:"issue"`
}

// An Identifier is a name/id pair.
type Identifier struct {
	Name string `json:"name,omitempty"`
	Id   int    `json:"id,omitempty"`
}

// A ValueField is an Identifier with an associated value.
type ValueField struct {
	Identifier
	Value string `json:"value,omitempty"`
}

// NewSession creates a new session for a Redmine server.
func NewSession(redmineUrl, username, password string) (Session, error) {
	session := Session{
		url:      redmineUrl,
		username: username,
		password: password,
	}

	user, err := session.GetUser()
	if err != nil {
		return session, err
	}

	log.Printf("got user: %s", user)
	session.apiKey = user.ApiKey

	return session, nil
}

// OpenSession opens an existing session for a Redmine server.
func OpenSession(redmineUrl, apiKey string) Session {
	session := Session{
		url:    redmineUrl,
		apiKey: apiKey,
	}
	return session
}

// Url returns the Redmine server URL for a Session.
func (session *Session) Url() string {
	return session.url
}

// ApiKey returns the API key a Session uses when communicating with Redmine.
func (session *Session) ApiKey() string {
	return session.apiKey
}

// IssueUrl returns the REST url for a particular issue.
func (session *Session) IssueUrl(issue Issue) string {
	return fmt.Sprintf("%s/issues/%d", session.url, issue.Id)
}

// GetUsers returns an arary of all Redmine users
func (session *Session) GetUsers() ([]User, error) {

	data, err := session.get("/users.json", nil)

	if err != nil {
		return nil, err
	}

	var list struct {
		Users []User `json:"users"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&list); err != nil {
		return nil, err
	}

	var users []User
	users = append(users, list.Users...)

	return users, nil
}

// GetUser returns account data for the user a Session was created for.
func (session *Session) GetUser() (user User, err error) {
	var data []byte
	if data, err = session.get("/users/current.json", nil); err != nil {
		return
	}

	var u struct {
		User User `json:"user"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if err = dec.Decode(&u); err != nil {
		return
	}

	user = u.User
	return
}

// CreateUser creates new Redmine user and returns it
func (session *Session) CreateUser(user User) (newUser User, err error) {
	log.Printf("Creating user %v", user.Login)

	data := map[string]interface{}{
		"user": user,
	}
	var resp []byte

	resp, err = session.post("/users.json", data)
	log.Printf("got response: %s", string(resp))

	if err != nil {
		return
	}

	var createdUser struct {
		User User `json:"user"`
	}
	dec := json.NewDecoder(bytes.NewReader(resp))
	if err = dec.Decode(&createdUser); err != nil {
		return
	}

	newUser = createdUser.User
	return
}

// GetGroups returns an array of all Redmine groups including custom fields values
func (session *Session) GetGroups() ([]Group, error) {

	data, err := session.get("/groups.json", nil)

	if err != nil {
		return nil, err
	}

	var list struct {
		Groups []Group `json:"groups"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&list); err != nil {
		return nil, err
	}

	var groups []Group
	groups = append(groups, list.Groups...)

	return groups, nil
}

// GetGroup returns a specific group
func (session *Session) GetGroup(groupId int) (group Group, err error) {
	var data []byte
	if data, err = session.get("/groups/"+strconv.Itoa(groupId)+".json", nil); err != nil {
		return
	}

	var g struct {
		Group Group `json:"group"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if err = dec.Decode(&g); err != nil {
		return
	}
	group = g.Group
	return
}

// CreateGroup creates new Redmine group and returns it
func (session *Session) CreateGroup(groupName string) (newGroup Group, err error) {
	log.Printf("Creating group %v", groupName)

	group := Group{
		Name: groupName,
	}
	data := map[string]interface{}{
		"group": group,
	}
	var resp []byte

	resp, err = session.post("/groups.json", data)
	log.Printf("got response: %s", string(resp))

	if err != nil {
		return
	}

	var createdGroup struct {
		Group Group `json:"group"`
	}
	dec := json.NewDecoder(bytes.NewReader(resp))
	if err = dec.Decode(&createdGroup); err != nil {
		return
	}

	newGroup = createdGroup.Group
	return
}

// AddUserToGroup adds user, specified by userId, to group, specified by groupId
func (session *Session) AddUserToGroup(userId int, groupId int) (err error) {

	data := map[string]interface{}{
		"user_id": strconv.Itoa(userId),
	}
	_, err = session.post("/groups/"+strconv.Itoa(groupId)+"/users.json", data)

	return
}

// GetIssues returns an array of all open issues assigned to the Session user.
func (session *Session) GetIssues() ([]Issue, error) {
	params := map[string]string{
		// "assigned_to_id": "me",
		"watcher_id": "me",
		"limit":      "100"}
	var issues []Issue
	offset := 0

	for {
		data, err := session.get("/issues.json", params)
		if err != nil {
			return nil, err
		}

		var list struct {
			Issues     []Issue `json:"issues"`
			Limit      int     `json:"limit"`
			Offset     int     `json:"offset"`
			TotalCount int     `json:"total_count"`
		}

		dec := json.NewDecoder(bytes.NewReader(data))
		err = dec.Decode(&list)
		if err != nil {
			return nil, err
		}

		issues = append(issues, list.Issues...)
		if len(issues) == list.TotalCount {
			break
		}

		offset += len(issues)
		params["offset"] = strconv.Itoa(offset)
	}

	return issues, nil
}

// GetIssue returns a specific issue.
func (session *Session) GetIssue(id int) (issue Issue, err error) {
	var data []byte
	if data, err = session.get("/issues/"+strconv.Itoa(id)+".json", nil); err != nil {
		return
	}

	var i struct {
		Issue Issue `json:"issue"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if err = dec.Decode(&i); err != nil {
		return
	}
	issue = i.Issue
	return
}

func (session *Session) UpdateIssue(id int, issue UpdateIssue) (err error) {
	log.Printf("Updating issue %v", issue)
	data := map[string]interface{}{
		"issue": issue,
	}
	var resp []byte
	resp, err = session.put("/issues/"+strconv.Itoa(id)+".json", data)
	log.Printf("got response: %s", string(resp))
	return err
}

// GetTimeEntries returns all time entries from a given number of days in the
// past until now.
func (session *Session) GetTimeEntries(daysBack int) ([]TimeEntry, error) {
	since := time.Now().AddDate(0, 0, -daysBack).Format("2006-01-02")
	until := time.Now().Format("2006-01-02")
	params := map[string]string{
		"user_id":  "me",
		"spent_on": "><" + since + "|" + until,
		"limit":    "100"}

	var entries []TimeEntry
	offset := 0

	for {
		data, err := session.get("/time_entries.json", params)
		if err != nil {
			return nil, err
		}

		var list struct {
			TimeEntries []TimeEntry `json:"time_entries"`
			Limit       int         `json:"limit"`
			Offset      int         `json:"offset"`
			TotalCount  int         `json:"total_count"`
		}

		dec := json.NewDecoder(bytes.NewReader(data))
		err = dec.Decode(&list)
		if err != nil {
			return nil, err
		}

		entries = append(entries, list.TimeEntries...)
		if len(entries) == list.TotalCount {
			break
		}

		offset += len(entries)
		params["offset"] = strconv.Itoa(offset)
	}

	return entries, nil
}

// GetProjects returns an array of all the projects the Session user belongs to.
func (session *Session) GetProjects() ([]Project, error) {
	params := map[string]string{
		"limit": "100"}

	var projects []Project
	offset := 0

	for {
		data, err := session.get("/projects.json", params)
		if err != nil {
			return nil, err
		}

		var list struct {
			Projects   []Project `json:"projects"`
			TotalCount int       `json:"total_count"`
			Offset     int       `json:"offset"`
			Limit      int       `json:"limit"`
		}

		dec := json.NewDecoder(bytes.NewReader(data))
		err = dec.Decode(&list)
		if err != nil {
			return nil, err
		}

		projects = append(projects, list.Projects...)
		if len(projects) == list.TotalCount {
			break
		}

		offset = len(projects)
		params["offset"] = strconv.Itoa(offset)
	}

	return projects, nil
}

// GetIssueStatuses returns an array of all the available issue statuses.
func (session *Session) GetIssueStatuses() ([]IssueStatus, error) {
	data, err := session.get("/issue_statuses.json", nil)
	if err != nil {
		return nil, err
	}

	var statuses struct {
		IssueStatuses []IssueStatus `json:"issue_statuses"`
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	err = dec.Decode(&statuses)
	if err != nil {
		return nil, err
	}

	return statuses.IssueStatuses, nil
}

// support /////////////////////////////////////////////////////////////

func toQueryString(params map[string]string) string {
	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	return values.Encode()
}

func (session *Session) request(method string, requestUrl string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequest(method, requestUrl, body)
	req.Header.Add("Content-Type", "application/json")

	if session.apiKey != "" {
		log.Printf("using api key: %s", session.apiKey)
		req.Header.Add("X-Redmine-API-Key", session.apiKey)
	} else {
		log.Printf("using auth key: %s:*****", session.username)
		req.SetBasicAuth(session.username, session.password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return content, fmt.Errorf(resp.Status)
	}

	return content, nil
}

func (session *Session) get(path string, params map[string]string) ([]byte, error) {
	requestUrl := session.url + path

	if params != nil {
		requestUrl += "?" + toQueryString(params)
	}

	log.Printf("GETing from URL: %s", requestUrl)
	return session.request("GET", requestUrl, nil)
}

func (session *Session) send(method, path string, data interface{}) ([]byte, error) {
	requestUrl := session.url + path

	var body []byte
	var err error

	if data != nil {
		body, err = json.Marshal(data)
		if err != nil {
			return nil, err
		}
	}

	log.Printf(method+"ing to URL %s: %s", requestUrl, string(body))
	return session.request(method, requestUrl, bytes.NewBuffer(body))
}

func (session *Session) post(path string, data interface{}) ([]byte, error) {
	return session.send("POST", path, data)
}

func (session *Session) put(path string, data interface{}) ([]byte, error) {
	return session.send("PUT", path, data)
}
