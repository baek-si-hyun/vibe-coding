package config

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type APIEndpoint struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

type Config struct {
	Host               string
	Port               string
	BackendDir         string
	DataRootDir        string
	DataDir            string
	TelegramDataDir    string
	NaverClientID      string
	NaverClientSecret  string
	KakaoRestAPIKey    string
	NewsAPIKey         string
	DARTFSSAPIKey      string
	DARTAPIBaseURL     string
	KRXAPIKey          string
	FREDAPIKey         string
	EIAAPIKey          string
	TwelveDataAPIKey   string
	PolygonAPIKey      string
	AlphaVantageAPIKey string
	MetalsAPIKey       string
	ExchangeRateAPIKey string
	APIEndpoints       map[string]APIEndpoint
}

func Load() Config {
	backendDir := resolveBackendDir()
	projectRoot := filepath.Dir(backendDir)

	loadDotEnvIfExists(filepath.Join(backendDir, ".env"))
	loadDotEnvIfExists(filepath.Join(projectRoot, ".env"))
	loadDotEnvIfExists(filepath.Join(projectRoot, "krx", ".env.local"))

	host := getenvDefault("NEWS_GO_HOST", getenvDefault("BACKEND_HOST", "0.0.0.0"))
	port := getenvDefault("NEWS_GO_PORT", getenvDefault("BACKEND_PORT", "5002"))
	dataRoot := strings.TrimSpace(os.Getenv("NEWS_DATA_ROOT_DIR"))
	if dataRoot == "" {
		dataRoot = filepath.Join(backendDir, "data")
	} else {
		dataRoot = absOrClean(dataRoot)
	}

	return Config{
		Host:               host,
		Port:               port,
		BackendDir:         backendDir,
		DataRootDir:        dataRoot,
		DataDir:            filepath.Join(dataRoot, "news"),
		TelegramDataDir:    filepath.Join(dataRoot, "telegram_chats"),
		NaverClientID:      strings.TrimSpace(os.Getenv("NAVER_CLIENT_ID")),
		NaverClientSecret:  strings.TrimSpace(os.Getenv("NAVER_CLIENT_SECRET")),
		KakaoRestAPIKey:    strings.TrimSpace(os.Getenv("KAKAO_REST_API_KEY")),
		NewsAPIKey:         strings.TrimSpace(getenvDefault("NEWSAPI_KEY", os.Getenv("NEXT_PUBLIC_NEWSAPI_API_KEY"))),
		DARTFSSAPIKey:      strings.TrimSpace(os.Getenv("DART_FSS_API_KEY")),
		DARTAPIBaseURL:     strings.TrimSpace(getenvDefault("DART_API_BASE_URL", "https://opendart.fss.or.kr/api")),
		KRXAPIKey:          strings.TrimSpace(getenvDefault("KRX_API_KEY", os.Getenv("KRX_OPENAPI_KEY"))),
		FREDAPIKey:         strings.TrimSpace(os.Getenv("FRED_API_KEY")),
		EIAAPIKey:          strings.TrimSpace(os.Getenv("EIA_API_KEY")),
		TwelveDataAPIKey:   strings.TrimSpace(os.Getenv("TWELVE_DATA_API_KEY")),
		PolygonAPIKey:      strings.TrimSpace(os.Getenv("POLYGON_API_KEY")),
		AlphaVantageAPIKey: strings.TrimSpace(os.Getenv("ALPHA_VANTAGE_API_KEY")),
		MetalsAPIKey:       strings.TrimSpace(os.Getenv("METALS_API_KEY")),
		ExchangeRateAPIKey: strings.TrimSpace(getenvDefault("EXCHANGERATE_HOST_API_KEY", os.Getenv("EXCHANGE_RATE_API_KEY"))),
		APIEndpoints:       loadAPIEndpoints(),
	}
}

func EndpointKeys(endpoints map[string]APIEndpoint) []string {
	keys := make([]string, 0, len(endpoints))
	for key := range endpoints {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func loadAPIEndpoints() map[string]APIEndpoint {
	raw := strings.TrimSpace(os.Getenv("KRX_API_ENDPOINTS"))
	if raw != "" {
		var parsed map[string]APIEndpoint
		if err := json.Unmarshal([]byte(raw), &parsed); err == nil && len(parsed) > 0 {
			return parsed
		}

		var parsedLoose map[string]map[string]any
		if err := json.Unmarshal([]byte(raw), &parsedLoose); err == nil && len(parsedLoose) > 0 {
			out := make(map[string]APIEndpoint, len(parsedLoose))
			for key, value := range parsedLoose {
				url, _ := value["url"].(string)
				name, _ := value["name"].(string)
				if strings.TrimSpace(url) == "" {
					continue
				}
				out[key] = APIEndpoint{
					URL:  strings.TrimSpace(url),
					Name: strings.TrimSpace(name),
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}

	base := strings.TrimSpace(getenvDefault("KRX_API_BASE_URL", os.Getenv("KRX_OPENAPI_BASE_URL")))
	if base == "" {
		base = "https://data.krx.co.kr/svc/apis"
	}
	base = strings.TrimRight(base, "/")

	return map[string]APIEndpoint{
		"kospi_daily":        {URL: base + "/sto/stk_bydd_trd", Name: "유가증권 일별매매정보"},
		"kosdaq_daily":       {URL: base + "/sto/ksq_bydd_trd", Name: "코스닥 일별매매정보"},
		"kospi_basic":        {URL: base + "/sto/stk_isu_base_info", Name: "유가증권 종목기본정보"},
		"kosdaq_basic":       {URL: base + "/sto/ksq_isu_base_info", Name: "코스닥 종목기본정보"},
		"smb_bond_daily":     {URL: base + "/bon/smb_bydd_trd", Name: "소액채권시장 일별매매정보"},
		"bond_index_daily":   {URL: base + "/idx/bon_dd_trd", Name: "채권지수 시세정보"},
		"gold_daily":         {URL: base + "/gen/gold_bydd_trd", Name: "금시장 일별매매정보"},
		"etf_daily":          {URL: base + "/etp/etf_bydd_trd", Name: "ETF 일별매매정보"},
		"kosdaq_index_daily": {URL: base + "/idx/kosdaq_dd_trd", Name: "KOSDAQ 시리즈 일별시세정보"},
		"krx_index_daily":    {URL: base + "/idx/krx_dd_trd", Name: "KRX 시리즈 일별시세정보"},
		"bond_daily":         {URL: base + "/bon/bnd_bydd_trd", Name: "일반채권시장 일별매매정보"},
	}
}

func resolveBackendDir() string {
	for _, key := range []string{"NEWS_GO_DIR", "NEWS_PROJECT_DIR", "NEWS_BACKEND_DIR"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return absOrClean(value)
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		return "backend-go"
	}
	candidates := make([]string, 0, 8)
	if strings.EqualFold(filepath.Base(wd), "backend-go") {
		candidates = append(candidates, wd)
	}
	candidates = append(candidates,
		filepath.Join(wd, "backend-go"),
		filepath.Join(wd, "..", "backend-go"),
		"backend-go",
	)
	for _, candidate := range candidates {
		info, statErr := os.Stat(candidate)
		if statErr == nil && info.IsDir() {
			return absOrClean(candidate)
		}
	}
	return absOrClean("backend-go")
}

func absOrClean(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return abs
}

func getenvDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func loadDotEnvIfExists(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
}
