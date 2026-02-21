package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html/template"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	mrand "math/rand"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type MemberProfile struct {
	Town         string
	Country      string
	Motto1       string
	Motto2       string
	Likes        string
	Dislikes     string
	GPS          string
	Enneagram1   int
	Enneagram2   int
	RegisteredAt time.Time
}

type User struct {
	Username     string
	PasswordHash string
	FullName     string
	Email        string
	Profile      MemberProfile
}

type memberRow struct {
	Member  *User
	TTT     *TttResult
	Status  string
	CanSend bool
}

type app struct {
	mu       sync.RWMutex
	users    map[string]*User
	results  map[string]*TttResult
	contacts map[string]map[string]bool
	sessions map[string]string
}

var gpsRe = regexp.MustCompile(`^\d+(?:\.\d+)?\s*[NnSs]\s*,?\s*\d+(?:\.\d+)?\s*[EeWw]$`)

func newApp() *app {
	return &app{
		users:    map[string]*User{},
		results:  map[string]*TttResult{},
		contacts: map[string]map[string]bool{},
		sessions: map[string]string{},
	}
}

func main() {
	a := newApp()
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.home)
	mux.HandleFunc("/register", a.register)
	mux.HandleFunc("/login", a.login)
	mux.HandleFunc("/logout", a.logout)
	mux.HandleFunc("/ttt", a.ttt)
	mux.HandleFunc("/search", a.search)
	mux.HandleFunc("/rcd", a.rcd)
	mux.HandleFunc("/status/", a.status)
	mux.HandleFunc("/members/plot.png", a.memberPlot)
	mux.HandleFunc("/static/style.css", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write([]byte(css))
	})
	log.Println("PbT listening on http://127.0.0.1:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func (a *app) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if u := a.currentUser(r); u != nil {
		http.Redirect(w, r, "/status/"+u.Username, http.StatusSeeOther)
		return
	}
	a.render(w, r, "Home", homeTpl, map[string]any{})
}

func (a *app) register(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		a.render(w, r, "Register", registerTpl, map[string]any{})
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	if username == "" || password == "" {
		a.render(w, r, "Register", registerTpl, map[string]any{"Error": "Username and password are required."})
		return
	}
	gps := strings.TrimSpace(r.FormValue("gps_coordinates"))
	if gps != "" && !gpsRe.MatchString(gps) {
		a.render(w, r, "Register", registerTpl, map[string]any{"Error": "GPS coordinates must look like: 52.5N, 13.4E"})
		return
	}
	a.mu.Lock()
	if _, exists := a.users[username]; exists {
		a.mu.Unlock()
		a.render(w, r, "Register", registerTpl, map[string]any{"Error": "Username is already in use."})
		return
	}
	u := &User{
		Username:     username,
		PasswordHash: hashPassword(password),
		FullName:     strings.TrimSpace(r.FormValue("fullname")),
		Email:        strings.TrimSpace(r.FormValue("email")),
		Profile: MemberProfile{
			Town:         strings.TrimSpace(r.FormValue("town")),
			Country:      strings.TrimSpace(r.FormValue("country")),
			Motto1:       strings.TrimSpace(r.FormValue("motto1")),
			Motto2:       strings.TrimSpace(r.FormValue("motto2")),
			Likes:        strings.TrimSpace(r.FormValue("likes")),
			Dislikes:     strings.TrimSpace(r.FormValue("dislikes")),
			GPS:          gps,
			Enneagram1:   atoiDefault(r.FormValue("enneagramtype1"), 0),
			Enneagram2:   atoiDefault(r.FormValue("enneagramtype2"), 10),
			RegisteredAt: time.Now(),
		},
	}
	if u.PasswordHash == "" {
		a.mu.Unlock()
		http.Error(w, "password setup failed", http.StatusInternalServerError)
		return
	}
	a.users[username] = u
	a.mu.Unlock()
	a.setSession(w, r, username)
	http.Redirect(w, r, "/ttt", http.StatusSeeOther)
}

func (a *app) login(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		a.render(w, r, "Login", loginTpl, map[string]any{})
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	a.mu.RLock()
	u := a.users[strings.TrimSpace(r.FormValue("username"))]
	a.mu.RUnlock()
	if u == nil || !checkPassword(u.PasswordHash, r.FormValue("password")) {
		a.render(w, r, "Login", loginTpl, map[string]any{"Error": "Login failed."})
		return
	}
	a.setSession(w, r, u.Username)
	http.Redirect(w, r, "/status/"+u.Username, http.StatusSeeOther)
}

func (a *app) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("pbt_session"); err == nil {
		a.mu.Lock()
		delete(a.sessions, c.Value)
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "pbt_session", Value: "", MaxAge: -1, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *app) ttt(w http.ResponseWriter, r *http.Request) {
	u := a.requireAuth(w, r)
	if u == nil {
		return
	}
	if r.Method == http.MethodGet {
		a.render(w, r, "TTT", tttTpl, map[string]any{"Questions": questions})
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	answers := map[int]string{}
	for i, q := range questions {
		v := r.FormValue(fmt.Sprintf("q%d", i))
		if v == q.AKey || v == q.BKey {
			answers[i] = v
		}
	}
	result := evaluateAnswers(answers)
	if result == nil {
		a.render(w, r, "TTT", tttTpl, map[string]any{"Questions": questions, "Error": "Please answer at least five questions per MBTI dimension."})
		return
	}
	a.mu.Lock()
	a.results[u.Username] = result
	a.mu.Unlock()
	a.render(w, r, "TTT Result", tttResultTpl, map[string]any{"Result": result})
}

func (a *app) search(w http.ResponseWriter, r *http.Request) {
	viewer := a.requireAuth(w, r)
	if viewer == nil {
		return
	}
	data := map[string]any{"XVar": "ei", "YVar": "sn"}
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		xVar := r.FormValue("x_var")
		yVar := r.FormValue("y_var")
		if xVar == "" {
			xVar = "ei"
		}
		if yVar == "" {
			yVar = "sn"
		}
		data["XVar"], data["YVar"] = xVar, yVar
		if r.FormValue("motto_contains") == "" && r.FormValue("ttt_types") == "" && r.FormValue("only_not_contacts") == "" && r.FormValue("in_my_country") == "" && r.FormValue("max_km") == "" {
			data["Error"] = "Select at least one filter."
		} else {
			data["Members"] = a.filteredMembers(viewer, r)
		}
	}
	a.render(w, r, "Search", searchTpl, data)
}

func (a *app) filteredMembers(viewer *User, r *http.Request) []memberRow {
	a.mu.RLock()
	list := make([]*User, 0, len(a.users))
	for _, u := range a.users {
		if u.Username != viewer.Username {
			list = append(list, u)
		}
	}
	a.mu.RUnlock()
	if r.FormValue("only_not_contacts") != "" {
		f := list[:0]
		for _, u := range list {
			if a.contactStatus(viewer.Username, u.Username) == "no_contact" {
				f = append(f, u)
			}
		}
		list = f
	}
	if r.FormValue("in_my_country") != "" {
		f := list[:0]
		for _, u := range list {
			if u.Profile.Country == viewer.Profile.Country {
				f = append(f, u)
			}
		}
		list = f
	}
	if s := strings.ToLower(strings.TrimSpace(r.FormValue("motto_contains"))); s != "" {
		f := list[:0]
		for _, u := range list {
			if strings.Contains(strings.ToLower(u.Profile.Motto1), s) || strings.Contains(strings.ToLower(u.Profile.Motto2), s) {
				f = append(f, u)
			}
		}
		list = f
	}
	if raw := strings.TrimSpace(r.FormValue("ttt_types")); raw != "" {
		types := map[string]bool{}
		for _, part := range strings.Split(raw, ",") {
			t := strings.ToUpper(strings.TrimSpace(part))
			if t != "" {
				types[t] = true
			}
		}
		f := list[:0]
		a.mu.RLock()
		for _, u := range list {
			if t := a.results[u.Username]; t != nil && types[t.Type] {
				f = append(f, u)
			}
		}
		a.mu.RUnlock()
		list = f
	}
	if maxKm, err := strconv.Atoi(strings.TrimSpace(r.FormValue("max_km"))); err == nil && maxKm > 0 {
		f := list[:0]
		for _, u := range list {
			d := distanceKm(viewer.Profile.GPS, u.Profile.GPS)
			if d == nil || *d <= float64(maxKm) {
				f = append(f, u)
			}
		}
		list = f
	}
	return a.annotateMembers(viewer.Username, list)
}

func (a *app) rcd(w http.ResponseWriter, r *http.Request) {
	viewer := a.requireAuth(w, r)
	if viewer == nil {
		return
	}
	_ = r.ParseForm()
	action := r.FormValue("action")
	for _, username := range r.Form["members"] {
		if username == viewer.Username {
			continue
		}
		a.mu.RLock()
		_, ok := a.users[username]
		a.mu.RUnlock()
		if !ok {
			continue
		}
		status := a.contactStatus(viewer.Username, username)
		switch {
		case action == "send" && (status == "no_contact" || status == "RCD_received"):
			a.setContact(viewer.Username, username, true)
		case action == "accept" && status == "RCD_received":
			a.setContact(viewer.Username, username, true)
		case action == "reject" && status == "RCD_received":
			a.setContact(username, viewer.Username, false)
		}
	}
	next := r.FormValue("next")
	if next == "" {
		next = "/status/" + viewer.Username
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (a *app) status(w http.ResponseWriter, r *http.Request) {
	viewer := a.requireAuth(w, r)
	if viewer == nil {
		return
	}
	username := strings.TrimPrefix(r.URL.Path, "/status/")
	a.mu.RLock()
	member := a.users[username]
	a.mu.RUnlock()
	if member == nil {
		http.NotFound(w, r)
		return
	}
	own := member.Username == viewer.Username
	msg := ""
	if r.Method == http.MethodPost && own {
		_ = r.ParseForm()
		gps := strings.TrimSpace(r.FormValue("gps_coordinates"))
		if gps != "" && !gpsRe.MatchString(gps) {
			msg = "GPS coordinates must look like: 52.5N, 13.4E"
		} else {
			a.mu.Lock()
			member.FullName = strings.TrimSpace(r.FormValue("fullname"))
			member.Email = strings.TrimSpace(r.FormValue("email"))
			member.Profile.Town = strings.TrimSpace(r.FormValue("town"))
			member.Profile.Country = strings.TrimSpace(r.FormValue("country"))
			member.Profile.Motto1 = strings.TrimSpace(r.FormValue("motto1"))
			member.Profile.Motto2 = strings.TrimSpace(r.FormValue("motto2"))
			member.Profile.Likes = strings.TrimSpace(r.FormValue("likes"))
			member.Profile.Dislikes = strings.TrimSpace(r.FormValue("dislikes"))
			member.Profile.GPS = gps
			member.Profile.Enneagram1 = atoiDefault(r.FormValue("enneagramtype1"), 0)
			member.Profile.Enneagram2 = atoiDefault(r.FormValue("enneagramtype2"), 10)
			a.mu.Unlock()
			msg = "Profile updated."
		}
	}
	relation := a.contactStatus(viewer.Username, member.Username)
	showContact := own || relation == "in_contact"
	a.mu.RLock()
	ttt := a.results[member.Username]
	others := make([]*User, 0, len(a.users)-1)
	for _, u := range a.users {
		if u.Username != viewer.Username {
			others = append(others, u)
		}
	}
	a.mu.RUnlock()
	inContact, sent, received := []*User{}, []*User{}, []*User{}
	for _, u := range others {
		s := a.contactStatus(viewer.Username, u.Username)
		switch s {
		case "in_contact":
			inContact = append(inContact, u)
		case "RCD_sent":
			sent = append(sent, u)
		case "RCD_received":
			received = append(received, u)
		}
	}
	sortUsers(inContact)
	sortUsers(sent)
	sortUsers(received)
	a.render(w, r, "Status", statusTpl, map[string]any{
		"Member":      member,
		"Profile":     member.Profile,
		"Own":         own,
		"Relation":    relation,
		"ShowContact": showContact,
		"TTT":         ttt,
		"InContact":   inContact,
		"Sent":        sent,
		"Received":    received,
		"Message":     msg,
	})
}

func (a *app) memberPlot(w http.ResponseWriter, r *http.Request) {
	viewer := a.requireAuth(w, r)
	if viewer == nil {
		return
	}
	parts := strings.Split(r.URL.Query().Get("members"), ",")
	users := []*User{}
	a.mu.RLock()
	for _, p := range parts {
		if u := a.users[strings.TrimSpace(p)]; u != nil {
			users = append(users, u)
		}
	}
	if !containsUser(users, viewer.Username) {
		users = append(users, viewer)
	}
	xVar, yVar := r.URL.Query().Get("x"), r.URL.Query().Get("y")
	if xVar == "" {
		xVar = "ei"
	}
	if yVar == "" {
		yVar = "sn"
	}
	resCopy := map[string]*TttResult{}
	for k, v := range a.results {
		resCopy[k] = v
	}
	a.mu.RUnlock()

	img := image.NewRGBA(image.Rect(0, 0, 480, 360))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	for x := 40; x <= 460; x++ {
		img.Set(x, 20, color.Black)
		img.Set(x, 320, color.Black)
	}
	for y := 20; y <= 320; y++ {
		img.Set(40, y, color.Black)
		img.Set(460, y, color.Black)
	}
	for x := 40; x <= 460; x++ {
		img.Set(x, 170, color.RGBA{153, 153, 153, 255})
	}
	for y := 20; y <= 320; y++ {
		img.Set(250, y, color.RGBA{153, 153, 153, 255})
	}
	rng := mrand.New(mrand.NewSource(7))
	for _, u := range users {
		result := resCopy[u.Username]
		x := variable(result, xVar) + rng.Float64()*0.66 - 0.33
		y := variable(result, yVar) + rng.Float64()*0.66 - 0.33
		px := int(250 + x*12)
		py := int(170 - y*12)
		drawDot(img, px, py, statusColor(a.contactStatus(viewer.Username, u.Username)))
	}
	w.Header().Set("Content-Type", "image/png")
	_ = png.Encode(w, img)
}

func (a *app) annotateMembers(viewer string, members []*User) []memberRow {
	sortUsers(members)
	rows := make([]memberRow, 0, len(members))
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, m := range members {
		status := a.contactStatusUnlocked(viewer, m.Username)
		rows = append(rows, memberRow{
			Member:  m,
			TTT:     a.results[m.Username],
			Status:  status,
			CanSend: status == "no_contact" || status == "RCD_received",
		})
	}
	return rows
}

func sortUsers(users []*User) {
	sort.Slice(users, func(i, j int) bool { return users[i].Username < users[j].Username })
}

func (a *app) contactStatus(viewer, other string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.contactStatusUnlocked(viewer, other)
}

func (a *app) contactStatusUnlocked(viewer, other string) string {
	if viewer == other {
		return "self"
	}
	ab := a.contacts[viewer] != nil && a.contacts[viewer][other]
	ba := a.contacts[other] != nil && a.contacts[other][viewer]
	if ab && ba {
		return "in_contact"
	}
	if ab {
		return "RCD_sent"
	}
	if ba {
		return "RCD_received"
	}
	return "no_contact"
}

func (a *app) setContact(from, to string, value bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if value {
		if a.contacts[from] == nil {
			a.contacts[from] = map[string]bool{}
		}
		a.contacts[from][to] = true
		return
	}
	if a.contacts[from] != nil {
		delete(a.contacts[from], to)
	}
}

func distanceKm(a, b string) *float64 {
	la, loa, okA := parseGPS(a)
	lb, lob, okB := parseGPS(b)
	if !okA || !okB {
		return nil
	}
	const earthRadiusKm = 6371.0
	lat1 := la * math.Pi / 180.0
	lat2 := lb * math.Pi / 180.0
	dLat := (lb - la) * math.Pi / 180.0
	dLon := (lob - loa) * math.Pi / 180.0
	sinLat := math.Sin(dLat / 2)
	sinLon := math.Sin(dLon / 2)
	h := sinLat*sinLat + math.Cos(lat1)*math.Cos(lat2)*sinLon*sinLon
	d := 2 * earthRadiusKm * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
	return &d
}

func parseGPS(s string) (lat, lon float64, ok bool) {
	if s == "" {
		return 0, 0, false
	}
	clean := strings.Fields(strings.ReplaceAll(s, ",", " "))
	if len(clean) != 4 {
		return 0, 0, false
	}
	la, err1 := strconv.ParseFloat(clean[0], 64)
	lo, err2 := strconv.ParseFloat(clean[2], 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	if strings.EqualFold(clean[1], "S") {
		la = -la
	}
	if strings.EqualFold(clean[3], "W") {
		lo = -lo
	}
	return la, lo, true
}

func variable(result *TttResult, key string) float64 {
	if result == nil {
		return 0
	}
	switch key {
	case "ei":
		return float64(result.EI)
	case "sn":
		return float64(result.SN)
	case "tf":
		return float64(result.TF)
	case "jp":
		return float64(result.JP)
	default:
		return 0
	}
}

func drawDot(img *image.RGBA, x, y int, c color.Color) {
	for dx := -4; dx <= 4; dx++ {
		for dy := -4; dy <= 4; dy++ {
			if dx*dx+dy*dy <= 16 {
				px, py := x+dx, y+dy
				if px >= 0 && py >= 0 && px < img.Bounds().Dx() && py < img.Bounds().Dy() {
					img.Set(px, py, c)
				}
			}
		}
	}
}

func statusColor(status string) color.Color {
	switch status {
	case "self":
		return color.RGBA{0, 0, 255, 255}
	case "RCD_sent":
		return color.RGBA{255, 0, 0, 255}
	case "in_contact":
		return color.RGBA{0, 128, 0, 255}
	case "RCD_received":
		return color.RGBA{128, 0, 128, 255}
	default:
		return color.Black
	}
}

func (a *app) currentUser(r *http.Request) *User {
	c, err := r.Cookie("pbt_session")
	if err != nil || c.Value == "" {
		return nil
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.users[a.sessions[c.Value]]
}

func (a *app) requireAuth(w http.ResponseWriter, r *http.Request) *User {
	u := a.currentUser(r)
	if u == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
	return u
}

func (a *app) setSession(w http.ResponseWriter, r *http.Request, username string) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		http.Error(w, "session setup failed", http.StatusInternalServerError)
		return
	}
	token := hex.EncodeToString(b)
	a.mu.Lock()
	a.sessions[token] = username
	a.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "pbt_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil})
}

func hashPassword(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return ""
	}
	return string(hash)
}

func checkPassword(stored, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)) == nil
}

func atoiDefault(s string, d int) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return d
	}
	return v
}

func containsUser(users []*User, username string) bool {
	for _, u := range users {
		if u.Username == username {
			return true
		}
	}
	return false
}

func (a *app) render(w http.ResponseWriter, r *http.Request, title, body string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	if _, ok := data["CurrentUser"]; !ok {
		data["CurrentUser"] = a.currentUser(r)
	}
	data["Title"] = title
	tpl := template.Must(template.New("base").Parse(baseTpl + body))
	_ = tpl.ExecuteTemplate(w, "base", data)
}

const baseTpl = `{{define "base"}}<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>PbT</title><link rel="stylesheet" href="/static/style.css"></head><body><header><h1>PbT - People by Temperament</h1><nav>{{if .CurrentUser}}<a href="/status/{{.CurrentUser.Username}}">Status</a><a href="/ttt">TTT</a><a href="/search">Search</a><a href="/logout">Logout</a>{{else}}<a href="/">Home</a><a href="/register">Register</a><a href="/login">Login</a>{{end}}</nav></header><main>{{template "body" .}}</main></body></html>{{end}}`

const homeTpl = `{{define "body"}}<p>Find members by temperament, values, and interests.</p><p><a href="/register">Register now</a> or <a href="/login">log in</a>.</p>{{end}}`

const registerTpl = `{{define "body"}}<h2>Registration</h2>{{if .Error}}<p class="msg">{{.Error}}</p>{{end}}<form method="post"><label>Full name <input name="fullname" required></label><label>Email <input type="email" name="email" required></label><label>Town <input name="town" required></label><label>Country <input name="country" required></label><label>Life motto <input name="motto1" required></label><label>Secondary motto <input name="motto2"></label><label>Likes (comma-separated) <input name="likes"></label><label>Dislikes (comma-separated) <input name="dislikes"></label><label>GPS coordinates <input name="gps_coordinates" placeholder="52.5N, 13.4E"></label><label>Primary Enneagram <input name="enneagramtype1" value="0"></label><label>Secondary Enneagram <input name="enneagramtype2" value="10"></label><label>Username <input name="username" required></label><label>Password <input type="password" name="password" required></label><button type="submit">Register</button></form>{{end}}`

const loginTpl = `{{define "body"}}<h2>Login</h2>{{if .Error}}<p class="msg">{{.Error}}</p>{{end}}<form method="post"><label>Username <input name="username" required></label><label>Password <input type="password" name="password" required></label><button type="submit">Login</button></form>{{end}}`

const tttTpl = `{{define "body"}}<h2>Trivial Temperament Test</h2>{{if .Error}}<p class="msg">{{.Error}}</p>{{end}}<form method="post">{{range $i, $q := .Questions}}<fieldset><legend>{{$i}}. {{$q.Prompt}}</legend><label><input type="radio" name="q{{$i}}" value="{{$q.AKey}}"> {{$q.AText}}</label><label><input type="radio" name="q{{$i}}" value="{{$q.BKey}}"> {{$q.BText}}</label><label><input type="radio" name="q{{$i}}" value=""> no answer</label></fieldset>{{end}}<button type="submit">Evaluate</button></form>{{end}}`

const tttResultTpl = `{{define "body"}}<h2>Your TTT result</h2><p>Result: <strong>{{.Result.Result}}</strong></p><p>MBTI type: <strong>{{.Result.Type}}</strong></p><p>Keirsey temperament: <strong>{{.Result.Temperament}}</strong></p><p><a href="/search">Continue to member search</a></p>{{end}}`

const searchTpl = `{{define "body"}}<h2>Search for members</h2>{{if .Error}}<p class="msg">{{.Error}}</p>{{end}}<form method="post"><label><input type="checkbox" name="only_not_contacts" value="1"> Only members not yet my contacts</label><label><input type="checkbox" name="in_my_country" value="1"> Only members in my country</label><label>Max distance (km) <input name="max_km"></label><label>Motto contains <input name="motto_contains"></label><label>TTT types (comma-separated, e.g. INTP,ESTJ) <input name="ttt_types"></label><label>Plot X variable <select name="x_var"><option value="ei">E-I</option><option value="sn">S-N</option><option value="tf">T-F</option><option value="jp">J-P</option></select></label><label>Plot Y variable <select name="y_var"><option value="ei">E-I</option><option value="sn">S-N</option><option value="tf">T-F</option><option value="jp">J-P</option></select></label><button type="submit">Search</button></form>{{if .Members}}<h3>Results</h3><img src="/members/plot.png?members={{range $i, $row := .Members}}{{if $i}},{{end}}{{$row.Member.Username}}{{end}}&x={{.XVar}}&y={{.YVar}}" alt="Member overview plot"><form method="post" action="/rcd"><input type="hidden" name="next" value="/search"><table><tr><th></th><th>User</th><th>Town</th><th>Country</th><th>Mottos</th><th>TTT</th><th>Enneagram</th><th>Status</th></tr>{{range .Members}}<tr><td>{{if .CanSend}}<input type="checkbox" name="members" value="{{.Member.Username}}">{{end}}</td><td><a href="/status/{{.Member.Username}}">{{.Member.Username}}</a></td><td>{{.Member.Profile.Town}}</td><td>{{.Member.Profile.Country}}</td><td>{{.Member.Profile.Motto1}}{{if .Member.Profile.Motto2}}, {{.Member.Profile.Motto2}}{{end}}</td><td>{{if .TTT}}{{.TTT.Type}}{{else}}-{{end}}</td><td>{{.Member.Profile.Enneagram1}}/{{.Member.Profile.Enneagram2}}</td><td>{{.Status}}</td></tr>{{end}}</table><button type="submit" name="action" value="send">Send RCD</button></form>{{end}}{{end}}`

const statusTpl = `{{define "body"}}<h2>Status page: {{.Member.Username}}</h2><p>Relationship status: {{.Relation}}</p>{{if .Message}}<p class="msg">{{.Message}}</p>{{end}}<h3>Profile</h3>{{if .Own}}<form method="post"><label>Full name <input name="fullname" value="{{.Member.FullName}}"></label><label>Email <input name="email" value="{{.Member.Email}}"></label><label>Town <input name="town" value="{{.Profile.Town}}"></label><label>Country <input name="country" value="{{.Profile.Country}}"></label><label>Motto 1 <input name="motto1" value="{{.Profile.Motto1}}"></label><label>Motto 2 <input name="motto2" value="{{.Profile.Motto2}}"></label><label>Likes <input name="likes" value="{{.Profile.Likes}}"></label><label>Dislikes <input name="dislikes" value="{{.Profile.Dislikes}}"></label><label>GPS <input name="gps_coordinates" value="{{.Profile.GPS}}"></label><label>Enneagram 1 <input name="enneagramtype1" value="{{.Profile.Enneagram1}}"></label><label>Enneagram 2 <input name="enneagramtype2" value="{{.Profile.Enneagram2}}"></label><button type="submit">Save profile</button></form>{{else}}{{if .ShowContact}}<p>Full name: {{.Member.FullName}}<br>Email: {{.Member.Email}}</p>{{else}}<p>Contact details are hidden until you are in contact.</p>{{end}}<p>Town: {{.Profile.Town}}, {{.Profile.Country}}</p><p>Mottos: {{.Profile.Motto1}}{{if .Profile.Motto2}} | {{.Profile.Motto2}}{{end}}</p>{{end}}<h3>Temperament Test</h3>{{if .TTT}}<p>{{.TTT.Result}} ({{.TTT.Type}})</p>{{else}}<p>No TTT result yet.</p>{{end}}{{if .Own}}<p><a href="/ttt">Take/re-take TTT</a></p>{{end}}{{if .Own}}<h3>In contact</h3><ul>{{range .InContact}}<li><a href="/status/{{.Username}}">{{.Username}}</a></li>{{else}}<li>None</li>{{end}}</ul><h3>RCD sent</h3><ul>{{range .Sent}}<li><a href="/status/{{.Username}}">{{.Username}}</a></li>{{else}}<li>None</li>{{end}}</ul><h3>RCD received</h3><form method="post" action="/rcd"><input type="hidden" name="next" value="/status/{{.Member.Username}}">{{range .Received}}<label><input type="checkbox" name="members" value="{{.Username}}"> {{.Username}}</label>{{else}}<p>None</p>{{end}}<button type="submit" name="action" value="accept">Accept selected</button><button type="submit" name="action" value="reject">Reject selected</button></form>{{end}}{{end}}`

const css = `body { font-family: sans-serif; margin: 0; background: #fafafa; }
header { background: #1f3b66; color: #fff; padding: 0.8rem 1rem; }
nav a { color: #fff; margin-right: 1rem; }
main { max-width: 960px; margin: 1rem auto; background: #fff; padding: 1rem; border: 1px solid #ddd; }
label { display: block; margin: 0.4rem 0; }
input, select { min-width: 18rem; }
fieldset { margin-bottom: 0.7rem; }
table { border-collapse: collapse; width: 100%; margin-top: 1rem; }
th, td { border: 1px solid #ddd; padding: 0.35rem; text-align: left; }
.msg { margin: 0.6rem auto; max-width: 960px; background: #fffae5; padding: 0.6rem; border: 1px solid #e3d69f; }`
