package thermia

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ---- Constants ----
const (
	clientID     = "09ea4903-9e95-45fe-ae1f-e3b7d32fa385"
	policy       = "b2c_1a_signuporsigninonline"
	redirectURI  = "https://online.thermia.se/login"
	scope        = clientID + " offline_access openid"

	baseB2C      = "https://thermialogin.b2clogin.com"
	tenantDomain = "thermialogin.onmicrosoft.com"

	authorizeURL = baseB2C + "/" + tenantDomain + "/" + policy + "/oauth2/v2.0/authorize"
	tokenURL     = baseB2C + "/" + tenantDomain + "/" + policy + "/oauth2/v2.0/token"
	selfURL      = baseB2C + "/" + tenantDomain + "/" + policy + "/SelfAsserted"
	confirmURL   = baseB2C + "/" + tenantDomain + "/" + policy + "/api/CombinedSigninAndSignup/confirmed"

	thermiaConfigURL = "https://online.thermia.se/api/configuration"

	REG_GROUP_TEMPERATURES          = "REG_GROUP_TEMPERATURES"
	REG_GROUP_OPERATIONAL_STATUS    = "REG_GROUP_OPERATIONAL_STATUS"
	REG_GROUP_OPERATIONAL_TIME      = "REG_GROUP_OPERATIONAL_TIME"
	REG_GROUP_OPERATIONAL_OPERATION = "REG_GROUP_OPERATIONAL_OPERATION"
	REG_GROUP_HOT_WATER             = "REG_GROUP_HOT_WATER"
)

var operationalStatusCandidates = []string{
	"REG_OPERATIONAL_STATUS_PRIORITY_BITMASK", "COMP_STATUS", "COMP_STATUS_ATEC", "COMP_STATUS_ITEC",
}

// ---- Public entrypoint ----
func FetchThermiaSummary(ctx context.Context, username, password string) (ThermiaSummary, error) {
	jar, _ := cookiejar.New(nil)
	hc := &http.Client{ Timeout: 30 * time.Second, Jar: jar }

	code, verifier, csrf, stateProps, cookies, err := startAuthorize(ctx, hc)
	if err != nil && !errors.Is(err, errNeedSelf) { return ThermiaSummary{}, err }
	if errors.Is(err, errNeedSelf) {
		if err := doSelfAsserted(ctx, hc, username, password, csrf, stateProps, cookies); err != nil { return ThermiaSummary{}, err }
		code, err = confirmAndGetCode(ctx, hc, csrf, stateProps, cookies)
		if err != nil { return ThermiaSummary{}, err }
	}
	token, err := exchangeCode(ctx, hc, code, verifier)
	if err != nil { return ThermiaSummary{}, err }

	cfg, _, err := getConfiguration(ctx, hc, token)
	if err != nil { return ThermiaSummary{}, err }
	apiBase := strings.TrimRight(cfg.APIBaseURL, "/")

	_, insts, err := getInstallations(ctx, hc, token, apiBase)
	if err != nil { return ThermiaSummary{}, err }
	if len(insts) == 0 { return ThermiaSummary{}, errors.New("no installations") }
	inst := insts[0]

	info, err := getInstallationInfo(ctx, hc, token, apiBase, inst.ID)
	if err != nil { return ThermiaSummary{}, err }
	status, err := getInstallationStatus(ctx, hc, token, apiBase, inst.ID)
	if err != nil { return ThermiaSummary{}, err }

	grpOperation, _ := getRegisterGroup(ctx, hc, token, apiBase, inst.ID, REG_GROUP_OPERATIONAL_OPERATION)
	grpStatus, _ := getRegisterGroup(ctx, hc, token, apiBase, inst.ID, REG_GROUP_OPERATIONAL_STATUS)
	grpTemps, _ := getRegisterGroup(ctx, hc, token, apiBase, inst.ID, REG_GROUP_TEMPERATURES)
	grpTime, _ := getRegisterGroup(ctx, hc, token, apiBase, inst.ID, REG_GROUP_OPERATIONAL_TIME)
	grpHot, _ := getRegisterGroup(ctx, hc, token, apiBase, inst.ID, REG_GROUP_HOT_WATER)

	// Alerts
	activeEvts, _ := getEvents(ctx, hc, token, apiBase, inst.ID, true)
	allEvts, _ := getEvents(ctx, hc, token, apiBase, inst.ID, false)
	activeTitles := uniqueTitles(activeEvts)
	allTitles := uniqueTitles(allEvts)
	archived := difference(allTitles, activeTitles)

	model := strings.TrimSpace(info.Model)
	modelID := strings.TrimSpace(info.Profile.Name)
	if model == "" { model = modelID }

	// Temperatures
	temps := deriveTemperatures(status, grpTemps)
	var outdoor *float64
	if v, ok := findValue(grpTemps, "REG_OUTDOOR_TEMPERATURE"); ok { outdoor = v
	} else if v, ok := findValue(grpTemps, "REG_OPER_DATA_OUTDOOR_TEMP_MA_SA"); ok { outdoor = v }

	tmap := map[string]float64{}
	if temps.Indoor != nil && *temps.Indoor < 100 { tmap["indoor"] = round1(*temps.Indoor) }
	if temps.SupplyLine != nil { tmap["supply_line"] = round1(*temps.SupplyLine) }
	if temps.DesiredSupplyLine != nil { tmap["desired_supply_line"] = round1(*temps.DesiredSupplyLine) }
	if temps.BufferTank != nil { tmap["buffer_tank"] = round1(*temps.BufferTank) }
	if temps.ReturnLine != nil { tmap["return_line"] = round1(*temps.ReturnLine) }
	if temps.HotWater != nil { tmap["hot_water"] = round1(*temps.HotWater) }
	if temps.BrineOut != nil { tmap["brine_out"] = round1(*temps.BrineOut) }
	if temps.BrineIn != nil { tmap["brine_in"] = round1(*temps.BrineIn) }
	if temps.Pool != nil { tmap["pool"] = round1(*temps.Pool) }
	if temps.CoolingTank != nil { tmap["cooling_tank"] = round1(*temps.CoolingTank) }
	if temps.CoolingSupply != nil { tmap["cooling_supply"] = round1(*temps.CoolingSupply) }
	if outdoor != nil { tmap["outdoor"] = round1(*outdoor) }

	// Modes
	opModeName, opModes, _ := extractOperationMode(grpOperation)

	// Statuses
	runStatuses, allStatuses := extractBitmaskStatuses(grpStatus, operationalStatusCandidates)
	runPower, allPower := extractBitmaskStatuses(grpStatus, []string{"COMP_POWER_STATUS"})

	// Hot water
	hs, hb := extractHotWaterSwitches(grpHot)

	// Op time
	opH := extractOperationalTime(grpTime)

	// last_online -> unix
	lastUnix := parseTimeToUnix(info.LastOnline)

	return ThermiaSummary{
		HeatpumpID: inst.ID,
		HeatpumpName: safe(info.Name, inst.Name),
		HeatpumpModel: model,
		Online: info.IsOnline,
		LastOnline: info.LastOnline,
		LastOnlineUnix: lastUnix,
		Temperatures: tmap,
		OperationModesAvailable: opModes,
		OperationMode: opModeName,
		OperationalStatusAvailable: allStatuses,
		OperationalStatusRunning: runStatuses,
		PowerStatusAvailable: allPower,
		PowerStatusRunning: runPower,
		HotWaterSwitch: hs,
		HotWaterBoost: hb,
		OperationalTimeHours: opH,
		ActiveAlerts: activeTitles,
		ArchivedAlerts: archived,
	}, nil
}

// ---- Auth flow ----
var errNeedSelf = errors.New("need SelfAsserted step")

func startAuthorize(ctx context.Context, hc *http.Client) (code, verifier, csrf, stateProps string, cookies []*http.Cookie, err error) {
	verifier = randomChallenge(43)
	challenge := pkceS256(verifier)

	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("scope", scope)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")

	req, _ := http.NewRequestWithContext(ctx, "GET", authorizeURL+"?"+q.Encode(), nil)
	res, err := hc.Do(req); if err != nil { return "", "", "", "", nil, err }
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	setJSON := extractSettings(string(body)); if setJSON == "" { return "", "", "", "", nil, errors.New("SETTINGS JSON not found") }
	var settings struct { TransId string `json:"transId"`; Csrf string `json:"csrf"` }
	if err := json.Unmarshal([]byte(setJSON), &settings); err != nil { return "", "", "", "", nil, err }
	parts := strings.Split(settings.TransId, "="); if len(parts) != 2 { return "", "", "", "", nil, errors.New("unexpected transId") }
	stateProps, csrf = parts[1], settings.Csrf

	if c := res.Request.URL.Query().Get("code"); c != "" { return c, verifier, csrf, stateProps, res.Cookies(), nil }
	return "", verifier, csrf, stateProps, res.Cookies(), errNeedSelf
}

func doSelfAsserted(ctx context.Context, hc *http.Client, username, password, csrf, stateProps string, cookies []*http.Cookie) error {
	form := url.Values{}
	form.Set("request_type", "RESPONSE")
	form.Set("signInName", username)
	form.Set("password", password)

	u, _ := url.Parse(selfURL)
	q := u.Query()
	q.Set("tx", "StateProperties="+stateProps)
	q.Set("p", "B2C_1A_SignUpOrSigninOnline")
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, "POST", u.String(), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-Csrf-Token", csrf)
	for _, c := range cookies { req.AddCookie(c) }

	res, err := hc.Do(req); if err != nil { return err }
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode/100 != 2 || strings.Contains(string(b), `"status":"400"`) {
		return fmt.Errorf("SelfAsserted %d: %s", res.StatusCode, string(b))
	}
	return nil
}

func confirmAndGetCode(ctx context.Context, hc *http.Client, csrf, stateProps string, cookies []*http.Cookie) (string, error) {
	u, _ := url.Parse(confirmURL)
	q := u.Query()
	q.Set("csrf_token", csrf)
	q.Set("tx", "StateProperties="+stateProps)
	q.Set("p", "B2C_1A_SignUpOrSigninOnline")
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	for _, c := range cookies { req.AddCookie(c) }
	res, err := hc.Do(req); if err != nil { return "", err }
	defer res.Body.Close()

	final := res.Request.URL
	if strings.HasPrefix(final.String(), redirectURI) {
		if code := final.Query().Get("code"); code != "" { return code, nil }
	}
	r2, err := hc.Get(final.String()); if err != nil { return "", err }
	defer r2.Body.Close()
	if strings.HasPrefix(r2.Request.URL.String(), redirectURI) {
		if code := r2.Request.URL.Query().Get("code"); code != "" { return code, nil }
	}
	return "", errors.New("no auth code returned")
}

func exchangeCode(ctx context.Context, hc *http.Client, code, verifier string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("redirect_uri", redirectURI)
	form.Set("scope", scope)
	form.Set("code", code)
	form.Set("code_verifier", verifier)

	req, _ := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	res, err := hc.Do(req); if err != nil { return "", err }
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 { return "", fmt.Errorf("token endpoint %d: %s", res.StatusCode, string(b)) }
	var tr struct{ AccessToken string `json:"access_token"` }
	if err := json.Unmarshal(b, &tr); err != nil { return "", err }
	if tr.AccessToken == "" { return "", errors.New("no access_token") }
	return tr.AccessToken, nil
}

// ---- API calls ----
func getConfiguration(ctx context.Context, hc *http.Client, token string) (Config, []byte, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", thermiaConfigURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	res, err := hc.Do(req); if err != nil { return Config{}, nil, err }
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 { return Config{}, b, fmt.Errorf("GET %s returned %d: %s", thermiaConfigURL, res.StatusCode, string(b)) }
	var cfg Config
	_ = json.Unmarshal(b, &cfg)
	return cfg, b, nil
}

func getInstallations(ctx context.Context, hc *http.Client, token, apiBase string) ([]byte, []Installation, error) {
	u := strings.TrimRight(apiBase, "/") + "/api/v1/installationsInfo"
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	res, err := hc.Do(req); if err != nil { return nil, nil, err }
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 { return b, nil, fmt.Errorf("GET %s returned %d: %s", u, res.StatusCode, string(b)) }
	var wrap struct{ Items []Installation `json:"items"` }
	if err := json.Unmarshal(b, &wrap); err == nil && len(wrap.Items) > 0 { return b, wrap.Items, nil }
	var arr []Installation
	if err := json.Unmarshal(b, &arr); err == nil { return b, arr, nil }
	return b, nil, nil
}

func getInstallationInfo(ctx context.Context, hc *http.Client, token, apiBase string, id int64) (InstallationInfo, error) {
	u := fmt.Sprintf("%s/api/v1/installations/%d", strings.TrimRight(apiBase, "/"), id)
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	res, err := hc.Do(req); if err != nil { return InstallationInfo{}, err }
	defer res.Body.Close()
	if res.StatusCode != 200 { b, _ := io.ReadAll(res.Body); return InstallationInfo{}, fmt.Errorf("GET %s returned %d: %s", u, res.StatusCode, string(b)) }
	var info InstallationInfo
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil { return InstallationInfo{}, err }
	return info, nil
}

func getInstallationStatus(ctx context.Context, hc *http.Client, token, apiBase string, id int64) (InstallationStatus, error) {
	u := fmt.Sprintf("%s/api/v1/installationstatus/%d/status", strings.TrimRight(apiBase, "/"), id)
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	res, err := hc.Do(req); if err != nil { return InstallationStatus{}, err }
	defer res.Body.Close()
	if res.StatusCode != 200 { b, _ := io.ReadAll(res.Body); return InstallationStatus{}, fmt.Errorf("GET %s returned %d: %s", u, res.StatusCode, string(b)) }
	var st InstallationStatus
	if err := json.NewDecoder(res.Body).Decode(&st); err != nil { return InstallationStatus{}, err }
	return st, nil
}

func getRegisterGroup(ctx context.Context, hc *http.Client, token, apiBase string, id int64, group string) ([]GroupItem, error) {
	u := fmt.Sprintf("%s/api/v1/Registers/Installations/%d/Groups/%s", strings.TrimRight(apiBase, "/"), id, group)
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	res, err := hc.Do(req); if err != nil { return nil, err }
	defer res.Body.Close()
	if res.StatusCode != 200 { return nil, nil }
	var items []GroupItem
	if err := json.NewDecoder(res.Body).Decode(&items); err != nil { return nil, err }
	return items, nil
}

func getEvents(ctx context.Context, hc *http.Client, token, apiBase string, id int64, onlyActive bool) ([]Event, error) {
	u := fmt.Sprintf("%s/api/v1/installation/%d/events?onlyActiveAlarms=%v", strings.TrimRight(apiBase, "/"), id, onlyActive)
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	res, err := hc.Do(req); if err != nil { return nil, err }
	defer res.Body.Close()
	if res.StatusCode != 200 { return nil, nil }
	var evts []Event
	if err := json.NewDecoder(res.Body).Decode(&evts); err != nil { return nil, err }
	return evts, nil
}

// ---- helpers & mappers ----
func extractSettings(html string) string {
	re := regexp.MustCompile(`var SETTINGS = ([\s\S]*?});`)
	m := re.FindStringSubmatch(html)
	if len(m) < 2 { return "" }
	return strings.TrimSpace(m[1])
}

func randomChallenge(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~"
	var sb strings.Builder; sb.Grow(n)
	x := time.Now().UnixNano()
	for i := 0; i < n; i++ { x = (x*1664525 + 1013904223) & 0x7fffffff; sb.WriteByte(alphabet[int(x)%len(alphabet)]) }
	return sb.String()
}
func pkceS256(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func findValue(items []GroupItem, registerName string) (*float64, bool) {
	for _, it := range items {
		if it.RegisterName == registerName && it.RegisterValue != nil { return it.RegisterValue, true }
	}
	return nil, false
}

func trimMode(s string) string {
	s = strings.TrimPrefix(s, "REG_VALUE_OPERATION_MODE_")
	s = strings.TrimPrefix(s, "REG_VALUE_")
	return s
}
func trimStatus(s string) string {
	for _, p := range []string{"REG_VALUE_", "COMP_VALUE_"} {
		if strings.HasPrefix(s, p) { return strings.TrimPrefix(s, p) }
	}
	return s
}
func safe(v, fb string) string { v = strings.TrimSpace(v); if v == "" { return fb }; return v }
func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10.0 }

func deriveTemperatures(st InstallationStatus, grp []GroupItem) tempsOut {
	out := tempsOut{
		Indoor: st.IndoorTemperature,
		HotWater: st.HotWaterTemperature,
		SupplyLine: st.SupplyLine,
		DesiredSupplyLine: func() *float64 {
			if st.DesiredSupplyLineTemperature != nil { return st.DesiredSupplyLineTemperature }
			if v, ok := findValue(grp, "REG_DESIRED_SUPPLY_LINE_TEMP"); ok { return v }
			if v, ok := findValue(grp, "REG_DESIRED_SUPPLY_LINE"); ok { return v }
			if v, ok := findValue(grp, "REG_DESIRED_SYS_SUPPLY_LINE_TEMP"); ok { return v }
			return nil
		}(),
		BufferTank: st.BufferTankTemperature,
		ReturnLine: func() *float64 {
			if st.ReturnLineTemperature != nil { return st.ReturnLineTemperature }
			if v, ok := findValue(grp, "REG_RETURN_LINE"); ok { return v }
			if v, ok := findValue(grp, "REG_OPER_DATA_RETURN"); ok { return v }
			return nil
		}(),
		BrineOut: st.BrineOutTemperature,
		BrineIn: st.BrineInTemperature,
		Pool: st.PoolTemperature,
		CoolingTank: st.CoolingTankTemperature,
		CoolingSupply: st.CoolingSupplyLineTemperature,
	}
	if out.Indoor == nil { if v, ok := findValue(grp, "REG_INDOOR_TEMPERATURE"); ok { out.Indoor = v } }
	if out.SupplyLine == nil { if v, ok := findValue(grp, "REG_SUPPLY_LINE"); ok { out.SupplyLine = v } }
	if out.BufferTank == nil { if v, ok := findValue(grp, "REG_OPER_DATA_BUFFER_TANK"); ok { out.BufferTank = v } }
	if out.BrineOut == nil { if v, ok := findValue(grp, "REG_BRINE_OUT"); ok { out.BrineOut = v } }
	if out.BrineIn == nil { if v, ok := findValue(grp, "REG_BRINE_IN"); ok { out.BrineIn = v } }
	if out.Pool == nil { if v, ok := findValue(grp, "REG_ACTUAL_POOL_TEMP"); ok { out.Pool = v } }
	if out.CoolingTank == nil { if v, ok := findValue(grp, "REG_COOL_SENSOR_TANK"); ok { out.CoolingTank = v } }
	if out.CoolingSupply == nil { if v, ok := findValue(grp, "REG_COOL_SENSOR_SUPPLY"); ok { out.CoolingSupply = v } }
	return out
}

func extractOperationMode(items []GroupItem) (current string, available []string, readOnly *bool) {
	for _, it := range items {
		if it.RegisterName == "REG_OPERATIONMODE" {
			available = []string{}
			for _, vn := range it.ValueNames { if vn.Visible { available = append(available, trimMode(vn.Name)) } }
			readOnly = &it.IsReadOnly
			if it.RegisterValue != nil {
				val := int(*it.RegisterValue + 0.00001)
				for _, vn := range it.ValueNames { if vn.Value == val { current = trimMode(vn.Name); break } }
			}
			break
		}
	}
	return
}

func extractBitmaskStatuses(items []GroupItem, registerNames []string) (running []string, available []string) {
	var match *GroupItem
	for _, rn := range registerNames {
		for i := range items { if items[i].RegisterName == rn { match = &items[i]; break } }
		if match != nil { break }
	}
	if match == nil { return nil, nil }
	for _, vn := range match.ValueNames { if vn.Visible { available = append(available, trimStatus(vn.Name)) } }
	if match.RegisterValue == nil { return []string{}, available }
	val := int(*match.RegisterValue + 0.00001)
	for _, vn := range match.ValueNames { if vn.Visible && (val & vn.Value) != 0 { running = append(running, trimStatus(vn.Name)) } }
	return
}

func extractHotWaterSwitches(items []GroupItem) (switchState *int, boostState *int) {
	for _, it := range items {
		switch it.RegisterName {
		case "REG__HOT_WATER_BOOST":
			if it.RegisterValue != nil { v := int(*it.RegisterValue + 0.00001); boostState = &v }
		case "REG_HOT_WATER_STATUS":
			if it.RegisterValue != nil { v := int(*it.RegisterValue + 0.00001); switchState = &v }
		}
	}
	return
}

func extractOperationalTime(items []GroupItem) map[string]int {
	keys := []string{ "REG_OPER_TIME_COMPRESSOR","REG_OPER_TIME_HEATING","REG_OPER_TIME_HOT_WATER","REG_OPER_TIME_IMM1","REG_OPER_TIME_IMM2","REG_OPER_TIME_IMM3" }
	out := map[string]int{}
	for _, k := range keys {
		for _, it := range items { if it.RegisterName == k && it.RegisterValue != nil { out[k] = int(*it.RegisterValue + 0.00001) } }
	}
	return out
}

// parse time (best-effort) -> unix seconds
func parseTimeToUnix(s string) int64 {
	if s == "" { return 0 }
	layouts := []string{
		"2006-01-02T15:04:05.000Z07:00",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
	}
	var t time.Time; var err error
	for _, l := range layouts {
		t, err = time.Parse(l, s)
		if err == nil { return t.Unix() }
	}
	return 0
}


func uniqueTitles(evts []Event) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, e := range evts {
		t := strings.TrimSpace(e.EventTitle)
		if t == "" || seen[t] { continue }
		seen[t] = true
		out = append(out, t)
	}
	return out
}

func difference(all, active []string) []string {
	set := map[string]struct{}{}
	for _, a := range active { set[a] = struct{}{} }
	out := []string{}
	for _, t := range all { if _, ok := set[t]; !ok { out = append(out, t) } }
	return out
}

