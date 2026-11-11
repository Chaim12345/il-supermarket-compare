// Package portals fetches store lists and full price files from the Israeli
// chains' price-transparency sites (חוק קידום התחרות בענף המזון).
//
// Covered portals:
//   - Shufersal (incl. Yesh Hesed, Universe, Deal, Express, Sheli)
//   - url.publishedprices.co.il (public per-chain usernames, empty password)
//   - Carrefour
package portals

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

const userAgent = "il-supermarket-compare/1.1 (personal price comparison)"

// Kind separates discount chains (priced per branch, cheap) from regular banners.
type Kind string

const (
	KindDiscount Kind = "discount"
	KindRegular  Kind = "regular"
)

// Chain is one retailer and how to reach its price files.
type Chain struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Kind   Kind   `json:"kind"`
	portal string
	user   string // publishedprices username
}

// Chains is the registry. Discount chains are priced branch by branch.
var Chains = []Chain{
	{Key: "shufersal", Name: "שופרסל", Kind: KindRegular, portal: "shufersal"},
	{Key: "carrefour", Name: "קרפור", Kind: KindRegular, portal: "carrefour"},
	{Key: "RamiLevi", Name: "רמי לוי", Kind: KindDiscount, portal: "publishedprices", user: "RamiLevi"},
	{Key: "osherad", Name: "אושר עד", Kind: KindDiscount, portal: "publishedprices", user: "osherad"},
	{Key: "yohananof", Name: "יוחננוף", Kind: KindDiscount, portal: "publishedprices", user: "yohananof"},
	{Key: "TivTaam", Name: "טיב טעם", Kind: KindRegular, portal: "publishedprices", user: "TivTaam"},
	{Key: "Keshet", Name: "קשת טעמים", Kind: KindDiscount, portal: "publishedprices", user: "Keshet"},
	{Key: "doralon", Name: "דור אלון (am:pm)", Kind: KindRegular, portal: "publishedprices", user: "doralon"},
	{Key: "freshmarket", Name: "פרשמרקט", Kind: KindRegular, portal: "publishedprices", user: "freshmarket"},
	{Key: "SalachD", Name: "סאלח דבאח", Kind: KindRegular, portal: "publishedprices", user: "SalachD"},
	{Key: "Stop_Market", Name: "סטופ מרקט", Kind: KindRegular, portal: "publishedprices", user: "Stop_Market"},
	{Key: "politzer", Name: "פוליצר", Kind: KindRegular, portal: "publishedprices", user: "politzer"},
}

// Store is a branch row from a chain's Stores file.
type Store struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	City     string `json:"city"`
	SubChain string `json:"subChain,omitempty"`
}

// Item is a product row from a PriceFull file.
type Item struct {
	Code         string
	Name         string
	Price        float64
	Quantity     float64
	Unit         string
	Weighted     bool
	UnitPrice    float64
	Manufacturer string
}

// Form is how a product is sold. A weighted item is loose produce sold by the
// kilogram; an item whose quantity is not a single unit is a sealed pack. This
// is what separates a loose tomato (per kg) from a can of diced tomatoes.
type Form int

const (
	Loose Form = iota
	Packaged
)

func (it Item) Form() Form {
	if !it.Weighted && it.Quantity > 0 && math.Abs(it.Quantity-1) > 1e-9 {
		return Packaged
	}
	return Loose
}

// ---------------- HTTP ----------------

var httpClient = &http.Client{Timeout: 90 * time.Second}

func get(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s: %s", rawURL, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// ---------------- decoding ----------------

// Decode inflates gzip if present and decodes UTF-8, UTF-8-BOM or UTF-16
// (the publishedprices portal serves UTF-16LE with a BOM).
func Decode(b []byte) (string, error) {
	if len(b) >= 2 && b[0] == 0x1f && b[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return "", err
		}
		defer zr.Close()
		raw, err := io.ReadAll(zr)
		if err != nil {
			return "", err
		}
		b = raw
	}
	switch {
	case len(b) >= 2 && b[0] == 0xff && b[1] == 0xfe:
		return utf16String(b[2:], false), nil
	case len(b) >= 2 && b[0] == 0xfe && b[1] == 0xff:
		return utf16String(b[2:], true), nil
	case len(b) >= 3 && b[0] == 0xef && b[1] == 0xbb && b[2] == 0xbf:
		return string(b[3:]), nil
	default:
		return string(b), nil
	}
}

func utf16String(b []byte, bigEndian bool) string {
	n := len(b) / 2
	u := make([]uint16, n)
	for i := 0; i < n; i++ {
		if bigEndian {
			u[i] = uint16(b[2*i])<<8 | uint16(b[2*i+1])
		} else {
			u[i] = uint16(b[2*i+1])<<8 | uint16(b[2*i])
		}
	}
	return string(utf16.Decode(u))
}

// ---------------- XML ----------------

var (
	reCache    sync.Map // tag -> *regexp.Regexp
	entRe      = regexp.MustCompile(`&(?:amp|lt|gt|quot|apos|nbsp);`)
	entMap     = map[string]string{"&amp;": "&", "&lt;": "<", "&gt;": ">", "&quot;": `"`, "&apos;": "'", "&nbsp;": " "}
	hrefRe     = regexp.MustCompile(`href="(https://[^"]+)"`)
	tokenRe    = regexp.MustCompile(`name="csrftoken" content="([^"]+)"`)
	digitsRe   = regexp.MustCompile(`[^0-9.]`)
	storeNumRe = regexp.MustCompile(`-(\d+)-`)
)

func unescapeXML(s string) string {
	return entRe.ReplaceAllStringFunc(s, func(m string) string {
		if v, ok := entMap[m]; ok {
			return v
		}
		return m
	})
}

func tagRe(tag string) *regexp.Regexp {
	if v, ok := reCache.Load(tag); ok {
		return v.(*regexp.Regexp)
	}
	re := regexp.MustCompile(`(?s)<` + tag + `(?:\s[^>]*)?>(.*?)</` + tag + `>`)
	reCache.Store(tag, re)
	return re
}

func tagText(xml, tag string) string {
	m := tagRe(tag).FindStringSubmatch(xml)
	if m == nil {
		return ""
	}
	return unescapeXML(strings.TrimSpace(m[1]))
}

func tagBlocks(xml, tag string) []string {
	ms := tagRe(tag).FindAllStringSubmatch(xml, -1)
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m[1])
	}
	return out
}

// ParseStores reads <Store> rows.
func ParseStores(xml string) []Store {
	blocks := tagBlocks(xml, "Store")
	out := make([]Store, 0, len(blocks))
	for _, b := range blocks {
		id := strings.TrimLeft(tagText(b, "StoreID"), "0")
		if id == "" {
			id = "0"
		}
		out = append(out, Store{
			ID:       id,
			Name:     tagText(b, "StoreName"),
			Address:  tagText(b, "Address"),
			City:     tagText(b, "City"),
			SubChain: tagText(b, "SubChainName"),
		})
	}
	return out
}

// ParseItems reads <Item> rows, skipping rows without a positive price.
func ParseItems(xml string) []Item {
	blocks := tagBlocks(xml, "Item")
	out := make([]Item, 0, len(blocks))
	for _, b := range blocks {
		price, err := strconv.ParseFloat(digitsRe.ReplaceAllString(tagText(b, "ItemPrice"), ""), 64)
		if err != nil || !(price > 0) {
			continue
		}
		qty, _ := strconv.ParseFloat(tagText(b, "Quantity"), 64)
		unitPrice, _ := strconv.ParseFloat(tagText(b, "UnitOfMeasurePrice"), 64)
		out = append(out, Item{
			Code:         tagText(b, "ItemCode"),
			Name:         strings.TrimSpace(tagText(b, "ItemName")),
			Price:        price,
			Quantity:     qty,
			Unit:         tagText(b, "UnitOfMeasure"),
			Weighted:     tagText(b, "bIsWeighted") == "1",
			UnitPrice:    unitPrice,
			Manufacturer: tagText(b, "ManufacturerName"),
		})
	}
	return out
}

// ---------------- Shufersal ----------------

const shufersalBase = "https://prices.shufersal.co.il/FileObject/UpdateCategory"

func shufersalLinks(ctx context.Context, cat int, store string) ([]string, error) {
	u := fmt.Sprintf("%s?catID=%d&storeId=%s", shufersalBase, cat, store)
	b, err := get(ctx, u)
	if err != nil {
		return nil, err
	}
	html := strings.ReplaceAll(string(b), "&amp;", "&")
	var out []string
	for _, m := range hrefRe.FindAllStringSubmatch(html, -1) {
		out = append(out, m[1])
	}
	return out, nil
}

func latestMatching(links []string, substr string) string {
	var matched []string
	for _, l := range links {
		if strings.Contains(l, substr) {
			matched = append(matched, l)
		}
	}
	if len(matched) == 0 {
		return ""
	}
	return matched[len(matched)-1]
}

// ---------------- publishedprices ----------------

const ppBase = "https://url.publishedprices.co.il"

type ppSession struct {
	client *http.Client
	token  string
}

func newPPSession(ctx context.Context, user string) (*ppSession, error) {
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, Timeout: 90 * time.Second}
	s := &ppSession{client: c}

	body, err := s.fetch(ctx, ppBase+"/login")
	if err != nil {
		return nil, err
	}
	tok := tokenRe.FindStringSubmatch(string(body))
	if tok == nil {
		return nil, fmt.Errorf("publishedprices: no csrftoken on login page")
	}
	form := url.Values{"username": {user}, "password": {""}, "csrftoken": {tok[1]}}
	if _, err := s.postForm(ctx, ppBase+"/login/user", form); err != nil {
		return nil, err
	}
	body, err = s.fetch(ctx, ppBase+"/file")
	if err != nil {
		return nil, err
	}
	tok = tokenRe.FindStringSubmatch(string(body))
	if tok == nil {
		return nil, fmt.Errorf("publishedprices login failed for %s", user)
	}
	s.token = tok[1]
	return s, nil
}

func (s *ppSession) fetch(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (s *ppSession) postForm(ctx context.Context, u string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (s *ppSession) list(ctx context.Context, search string) ([]string, error) {
	form := url.Values{
		"sEcho": {"1"}, "iDisplayStart": {"0"}, "iDisplayLength": {"100000"},
		"sSearch": {search}, "cd": {"/"}, "csrftoken": {s.token},
	}
	body, err := s.postForm(ctx, ppBase+"/file/json/dir", form)
	if err != nil {
		return nil, err
	}
	var res struct {
		AaData []struct {
			Fname string `json:"fname"`
		} `json:"aaData"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("publishedprices list: %w", err)
	}
	names := make([]string, 0, len(res.AaData))
	for _, f := range res.AaData {
		names = append(names, f.Fname)
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
	return names, nil
}

// ---------------- public API ----------------

// FetchStores returns every branch of a chain.
func FetchStores(ctx context.Context, c Chain) ([]Store, error) {
	switch c.portal {
	case "shufersal":
		links, err := shufersalLinks(ctx, 5, "0")
		if err != nil {
			return nil, err
		}
		u := latestMatching(links, "Stores")
		if u == "" && len(links) > 0 {
			u = links[0]
		}
		b, err := get(ctx, u)
		if err != nil {
			return nil, err
		}
		xml, err := Decode(b)
		if err != nil {
			return nil, err
		}
		return ParseStores(xml), nil
	case "carrefour":
		return carrefourStores(ctx)
	default:
		s, err := newPPSession(ctx, c.user)
		if err != nil {
			return nil, err
		}
		files, err := s.list(ctx, "Stores")
		if err != nil {
			return nil, err
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("publishedprices %s: no Stores file", c.user)
		}
		b, err := s.fetch(ctx, ppBase+"/file/d/"+url.PathEscape(files[len(files)-1]))
		if err != nil {
			return nil, err
		}
		xml, err := Decode(b)
		if err != nil {
			return nil, err
		}
		return ParseStores(xml), nil
	}
}

// FetchPriceFull returns the most recent full price file for one branch.
func FetchPriceFull(ctx context.Context, c Chain, storeID string) ([]Item, error) {
	switch c.portal {
	case "shufersal":
		links, err := shufersalLinks(ctx, 2, storeID)
		if err != nil {
			return nil, err
		}
		u := latestMatching(links, "PriceFull")
		if u == "" {
			return nil, fmt.Errorf("shufersal %s: no PriceFull published today", storeID)
		}
		b, err := get(ctx, u)
		if err != nil {
			return nil, err
		}
		xml, err := Decode(b)
		if err != nil {
			return nil, err
		}
		return ParseItems(xml), nil
	case "carrefour":
		return carrefourPriceFull(ctx, storeID)
	default:
		s, err := newPPSession(ctx, c.user)
		if err != nil {
			return nil, err
		}
		files, err := s.list(ctx, "PriceFull")
		if err != nil {
			return nil, err
		}
		needle := fmt.Sprintf("-%03d-", atoi(storeID))
		var matched []string
		for _, f := range files {
			if strings.Contains(f, needle) {
				matched = append(matched, f)
			}
		}
		if len(matched) == 0 {
			return nil, fmt.Errorf("publishedprices %s:%s: no PriceFull published today", c.user, storeID)
		}
		b, err := s.fetch(ctx, ppBase+"/file/d/"+url.PathEscape(matched[len(matched)-1]))
		if err != nil {
			return nil, err
		}
		xml, err := Decode(b)
		if err != nil {
			return nil, err
		}
		return ParseItems(xml), nil
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// ---------------- Carrefour ----------------

const carrefourBase = "https://prices.carrefour.co.il"

func carrefourIndex(ctx context.Context) (string, []string, error) {
	b, err := get(ctx, carrefourBase+"/")
	if err != nil {
		return "", nil, err
	}
	html := string(b)
	m := regexp.MustCompile(`const path = '(\d+)'`).FindStringSubmatch(html)
	if m == nil {
		return "", nil, fmt.Errorf("carrefour: could not read path")
	}
	names := regexp.MustCompile(`"name":"([^"]+)"`).FindAllStringSubmatch(html, -1)
	set := map[string]bool{}
	var list []string
	for _, n := range names {
		if !set[n[1]] {
			set[n[1]] = true
			list = append(list, n[1])
		}
	}
	sort.Strings(list)
	return m[1], list, nil
}

func carrefourStores(ctx context.Context) ([]Store, error) {
	path, names, err := carrefourIndex(ctx)
	if err != nil {
		return nil, err
	}
	var stores []string
	for _, n := range names {
		if strings.HasPrefix(n, "Stores") {
			stores = append(stores, n)
		}
	}
	if len(stores) == 0 {
		return nil, fmt.Errorf("carrefour: no Stores file")
	}
	b, err := get(ctx, fmt.Sprintf("%s/%s/%s", carrefourBase, path, stores[len(stores)-1]))
	if err != nil {
		return nil, err
	}
	xml, err := Decode(b)
	if err != nil {
		return nil, err
	}
	return ParseStores(xml), nil
}

func carrefourPriceFull(ctx context.Context, storeID string) ([]Item, error) {
	path, names, err := carrefourIndex(ctx)
	if err != nil {
		return nil, err
	}
	needle := fmt.Sprintf("-%03d-", atoi(storeID))
	var matched []string
	for _, n := range names {
		if strings.HasPrefix(n, "PriceFull") && strings.Contains(n, needle) {
			matched = append(matched, n)
		}
	}
	if len(matched) == 0 {
		return nil, fmt.Errorf("carrefour %s: no PriceFull published today", storeID)
	}
	b, err := get(ctx, fmt.Sprintf("%s/%s/%s", carrefourBase, path, matched[len(matched)-1]))
	if err != nil {
		return nil, err
	}
	xml, err := Decode(b)
	if err != nil {
		return nil, err
	}
	return ParseItems(xml), nil
}

// ---------------- city codes ----------------

// FetchCityCodes maps CBS locality code -> Hebrew name, because the chain files
// often give the city as a numeric code (e.g. 8600). Optional enrichment.
func FetchCityCodes(ctx context.Context) (map[string]string, error) {
	u := "https://data.gov.il/api/3/action/datastore_search?resource_id=5c78e9fa-c2e2-4771-93ff-7f400a12f7ba&limit=5000"
	b, err := get(ctx, u)
	if err != nil {
		return nil, err
	}
	var res struct {
		Result struct {
			Records []map[string]any `json:"records"`
		} `json:"result"`
	}
	if err := json.Unmarshal(b, &res); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, rec := range res.Result.Records {
		code := fmt.Sprintf("%v", rec["סמל_ישוב"])
		name, _ := rec["שם_ישוב"].(string)
		code = strings.TrimLeft(code, "0")
		if code != "" && name != "" {
			out[code] = strings.TrimSpace(name)
		}
	}
	return out, nil
}
