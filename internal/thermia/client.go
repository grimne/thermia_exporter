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
	clientID    = "09ea4903-9e95-45fe-ae1f-e3b7d32fa385"
	policy      = "b2c_1a_signuporsigninonline"
	redirectURI = "https://online.thermia.se/login"
	scope       = clientID + " offline_access openid"

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
	"REG_OPERATIONAL_STATUS_PRIORITY_BITMASK", // Atlas / priority-bitmask
	"COMP_STATUS",                             // Diplomat
	"COMP_STATUS_ATEC",                        // ATEC
	"COMP_STATUS_ITEC",                        // ITEC
}

// ---- Public entrypoint ----
func FetchThermiaSummary(ctx context.Context, username, password string) (ThermiaSummary, error) {
	jar, _ := cookiejar.New(nil)
	hc := &http.Client{Timeout: 30 * time.Second, Jar: jar}

	code, verifier, csrf, stateProps, cookies, err := startAuthorize(ctx, hc)
	if err != nil && !errors.Is(err, errNeedSelf) {
		return ThermiaSummary{}, err
	}
	if errors.Is(err, errNeedSelf) {
		if err := doSelfAsserted(ctx, hc, username, password, csrf, stateProps, cookies); err != nil {
			return ThermiaSummary{}, err
		}
		code, err = confirmAndGetCode(ctx, hc, csrf, stateProps, cookies)
		if err != nil {
			return ThermiaSummary{}, err
		}
	}
	token, err := exchangeCode(ctx, hc, code, verifier)
	if err != nil {
		return ThermiaSummary{}, err
	}

	cfg, _, err := getConfiguration(ctx, hc, token)
	if err != nil {
		return ThermiaSummary{}, err
	}
	apiBase := strings.TrimRight(cfg.APIBaseURL, "/")

	_, insts, err := getInstallations(ctx, hc, token, apiBase)
	if err != nil {
		return ThermiaSummary{}, err
	}
	if len(insts) == 0 {
		return ThermiaSummary{}, errors.New("no installations")
	}
	inst := insts[0]

	info, err := getInstallationInfo(ctx, hc, token, apiBase, inst.ID)
	if err != nil {
		return ThermiaSummary{}, err
	}
	status, err := getInstallationStatus(ctx, hc, token, apiBase, inst.ID)
	if err != nil {
		return ThermiaSummary{}, err
	}

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
	if model == "" {
		model = modelID
	}

	// Temperatures
	temps := deriveTemperatures(status, grpTemps)
	var outdoor *float64
	if v, ok := findValue(grpTemps, "REG_OUTDOOR_TEMPERATURE"); ok {
		outdoor = v
	} else if v, ok := findValue(grpTemps, "REG_OPER_DATA_OUTDOOR_TEMP_MA_SA"); ok {
		outdoor = v
	}

	tmap := map[string]float64{}
	if temps.Indoor != nil && *temps.Indoor < 100 {
		tmap["indoor"] = round1(*temps.Indoor)
	}
	if temps.SupplyLine != nil {
		tmap["supply_line"] = round1(*temps.SupplyLine)
	}
	if temps.DesiredSupplyLine != nil {
		tmap["desired_supply_line"] = round1(*temps.DesiredSupplyLine)
	}
	if temps.BufferTank != nil {
		tmap["buffer_tank"] = round1(*temps.BufferTank)
	}
	if temps.ReturnLine != nil {
		tmap["return_line"] = round1(*temps.ReturnLine)
	}
	if temps.HotWater != nil {
		tmap["hot_water"] = round1(*temps.HotWater)
	}
	if temps.BrineOut != nil {
		tmap["brine_out"] = round1(*temps.BrineOut)
	}
	if temps.BrineIn != nil {
		tmap["brine_in"] = round1(*temps.BrineIn)
	}
	if temps.Pool != nil {
		tmap["pool"] = round1(*temps.Pool)
	}
	if temps.CoolingTank != nil {
		tmap["cooling_tank"] = round1(*temps.CoolingTank)
	}
	if temps.CoolingSupply != nil {
		tmap["cooling_supply"] = round1(*temps.CoolingSupply)
	}
	if outdoor != nil {
		tmap["outdoor"] = round1(*outdoor)
	}

	// Modes
	opModeName, opModes, _ := extractOperationMode(grpOperation)

	// Operational/power statuses
	runStatuses, allStatuses := extractBitmaskStatuses(grpStatus, operationalStatusCandidates)
	runPower, allPower := extractBitmaskStatuses(grpStatus, []string{"COMP_POWER_STATUS"})

	// Hot water
	hs, hb := extractHotWaterSwitches(grpHot)

	// Op time
	opH := extractOperationalTime(grpTime)

	// last_online -> unix
	lastUnix := parseTimeToUnix(info.LastOnline)

	return ThermiaSummary{
		HeatpumpID:                inst.ID,
		HeatpumpName:              safe(info.Name, inst.Name),
		HeatpumpModel:             model,
		Online:                    info.IsOnline,
		LastOnline:                info.LastOnline,
		LastOnlineUnix:            lastUnix,
		Temperatures:              tmap,
		OperationModesAvailable:   opModes,
		OperationMode:             opModeName,
		OperationalStatusAvailable: allStatuses,
		OperationalStatusRunning:   runStatuses,
		PowerStatusAvailable:       allPower,
		PowerStatusRunning:         runPower,
		HotWaterSwitch:             hs,
		HotWaterBoost:              hb,
		OperationalTimeHours:       opH,
		ActiveAlerts:               activeTitles,
		ArchivedAlerts:             archived,
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
	res, err := hc.Do(req)
	if err != nil {
		return "", "", "", "", nil, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	setJSON := extractSettings(string(body))
	if setJSON == "" {
		return "", "", "", "", nil, errors.New("SETTINGS JSON not found")
	}
	var settings struct {
		TransId string `json:"transId"`
		Csrf    string `json:"csrf"`
	}
	if err := json.Unmarshal([]byte(setJSON), &settings); err != nil {
		return "", "", "", "", nil, err
	}
	parts := strings.Split(settings.TransId, "=")
	if len(parts) != 2 {
		return "", "", "", "", nil, errors.New("unexpected transId")
	}
	stateProps, csrf = parts[1], settings.Csrf

	if c := res.Request.URL.Query().Get("code"); c != "" {
		return c, verifier, csrf, stateProps, res.Cookies(), nil
	}
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
	for _, c := range cookies {
		req.AddCookie(c)
	}

	res, err := hc.Do(req)
	if err != nil {
		return err
	}
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
	for _, c := range cookies {
		req.AddCookie(c)
	}
	res, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	final := res.Request.URL
	if strings.HasPrefix(final.String(), redirectURI) {
		if code := final.Query().Get("code"); code != "" {
			return code, nil
		}
	}
	r2, err := hc.Get(final.String())
	if err != nil {
		return "", err
	}
	defer r2.Body.Close()
	if strings.HasPrefix(r2.Request.URL.String(), redirectURI) {
		if code := r2.Request.URL.Query().Get("code"); code != "" {
			return code, nil
		}
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
	res, err := hc.Do(req)
	if err != nil {
		return "", err
