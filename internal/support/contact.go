package support

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// supportLanguages are the preferred-language codes the support API accepts.
var supportLanguages = []string{"en-us", "zh-hans", "es-es", "fr-fr", "ja-jp", "ko-kr", "ru-ru", "pt-br", "it-it", "zh-tw", "de-de"}

// supportLanguage picks a support language from the locale environment.
func supportLanguage(getenv func(string) string) string {
	loc := ""
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := getenv(k); v != "" {
			loc = v
			break
		}
	}
	if i := strings.IndexAny(loc, ".@"); i >= 0 {
		loc = loc[:i]
	}
	parts := strings.FieldsFunc(strings.ToLower(loc), func(r rune) bool { return r == '_' || r == '-' })
	if len(parts) == 0 {
		return "en-us"
	}
	switch parts[0] {
	case "zh":
		for _, p := range parts[1:] {
			if p == "tw" || p == "hk" || p == "mo" || p == "hant" {
				return "zh-tw"
			}
		}
		return "zh-hans"
	case "es":
		return "es-es"
	case "fr":
		return "fr-fr"
	case "ja":
		return "ja-jp"
	case "ko":
		return "ko-kr"
	case "ru":
		return "ru-ru"
	case "pt":
		return "pt-br"
	case "it":
		return "it-it"
	case "de":
		return "de-de"
	}
	return "en-us"
}

// realCountry is the filter shared by countryAlpha3 and countryOptions.
func realCountry(r language.Region) (string, bool) {
	iso := r.ISO3()
	if !r.IsCountry() || len(iso) != 3 || iso == "ZZZ" || r.Canonicalize() != r || display.English.Regions().Name(r) == "" {
		return "", false
	}
	return iso, true
}

// countryAlpha3 turns an ISO 3166-1 alpha-2 or alpha-3 code into alpha-3.
func countryAlpha3(code string) (string, bool) {
	r, err := language.ParseRegion(code)
	if err != nil {
		return "", false
	}
	return realCountry(r)
}

// countryOptions lists "Name (ABC)" for every country, sorted by name.
func countryOptions() []string {
	var out []string
	for a := 'A'; a <= 'Z'; a++ {
		for b := 'A'; b <= 'Z'; b++ {
			r, err := language.ParseRegion(string([]rune{a, b}))
			if err != nil {
				continue
			}
			if iso, ok := realCountry(r); ok {
				out = append(out, display.English.Regions().Name(r)+" ("+iso+")")
			}
		}
	}
	sort.Strings(out)
	return out
}

type me struct {
	GivenName         string `json:"givenName"`
	Surname           string `json:"surname"`
	Mail              string `json:"mail"`
	UserPrincipalName string `json:"userPrincipalName"`
	UsageLocation     string `json:"usageLocation"`
}

var graphBaseURL = "https://graph.microsoft.com" // test seam

// fetchMe reads the signed-in user's profile from Microsoft Graph.
func fetchMe(ctx context.Context, cred azcore.TokenCredential) (me, error) {
	var m me
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{"https://graph.microsoft.com/.default"}})
	if err != nil {
		return m, fmt.Errorf("failed to get a Microsoft Graph token: %w", err)
	}
	u := graphBaseURL + "/v1.0/me?$select=" + url.QueryEscape("givenName,surname,mail,userPrincipalName,usageLocation")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return m, err
	}
	req.Header.Set("Authorization", "Bearer "+token.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return m, fmt.Errorf("failed to call Microsoft Graph: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return m, err
	}
	if resp.StatusCode != http.StatusOK {
		return m, fmt.Errorf("microsoft graph returned %s: %s", resp.Status, body)
	}
	err = json.Unmarshal(body, &m)
	return m, err
}

type contactDefaults struct{ FirstName, LastName, Email, Country, TimeZone, Language string }

// contactFromMe builds the new-ticket contact defaults from the Graph profile,
// the locale environment and the local IANA zone. Unknown values stay empty.
func contactFromMe(m me, getenv func(string) string, ianaZone string) contactDefaults {
	d := contactDefaults{FirstName: m.GivenName, LastName: m.Surname, Email: m.Mail, Language: supportLanguage(getenv)}
	if d.Email == "" && !strings.Contains(m.UserPrincipalName, "#EXT#") {
		d.Email = m.UserPrincipalName
	}
	d.Country, _ = countryAlpha3(m.UsageLocation)
	d.TimeZone, _ = windowsZone(ianaZone)
	return d
}
