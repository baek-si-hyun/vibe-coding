package news

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const trackedMinMarketCap int64 = 1_000_000_000_000

var trackedMarketDataDirs = []string{"kospi_daily", "kosdaq_daily"}

type companySnapshot struct {
	name      string
	asOf      string
	marketCap int64
}

func (s *Service) loadKeywords() ([]string, error) {
	return s.loadDomesticLargeCapKeywords()
}

func (s *Service) loadDomesticLargeCapKeywords() ([]string, error) {
	byName := map[string]string{}

	for _, dirName := range trackedMarketDataDirs {
		pattern := filepath.Join(s.cfg.DataRootDir, dirName, "*.csv")
		paths, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		snapshots := make([]companySnapshot, 0, len(paths))
		latestDate := ""
		for _, path := range paths {
			snapshot, snapErr := readLatestCompanySnapshot(path)
			if snapErr != nil {
				continue
			}
			snapshots = append(snapshots, snapshot)
			if snapshot.asOf > latestDate {
				latestDate = snapshot.asOf
			}
		}

		for _, snapshot := range snapshots {
			if snapshot.asOf != latestDate {
				continue
			}
			if snapshot.marketCap < trackedMinMarketCap {
				continue
			}
			name := strings.TrimSpace(snapshot.name)
			if name == "" {
				continue
			}
			byName[strings.ToLower(name)] = name
		}
	}

	if len(byName) == 0 {
		return nil, fmt.Errorf("KRX 저장 데이터에서 코스피/코스닥 시총 1조 이상 종목을 찾지 못했습니다. `%s` 아래 일봉 CSV를 먼저 준비하세요.", s.cfg.DataRootDir)
	}

	out := make([]string, 0, len(byName))
	for _, name := range byName {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

func readLatestCompanySnapshot(path string) (companySnapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return companySnapshot{}, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return companySnapshot{}, err
	}

	index := map[string]int{}
	for i, col := range header {
		index[strings.TrimPrefix(strings.TrimSpace(col), "\ufeff")] = i
	}

	dateIdx, ok := index["BAS_DD"]
	if !ok {
		return companySnapshot{}, fmt.Errorf("missing BAS_DD")
	}
	nameIdx, ok := index["ISU_NM"]
	if !ok {
		return companySnapshot{}, fmt.Errorf("missing ISU_NM")
	}
	capIdx, ok := index["MKTCAP"]
	if !ok {
		return companySnapshot{}, fmt.Errorf("missing MKTCAP")
	}

	best := companySnapshot{}
	for {
		row, readErr := r.Read()
		if readErr != nil {
			break
		}

		asOf := getCSVCell(row, dateIdx)
		name := getCSVCell(row, nameIdx)
		marketCap := parseMarketCap(getCSVCell(row, capIdx))
		if asOf == "" || name == "" || marketCap <= 0 {
			continue
		}

		if asOf > best.asOf || (asOf == best.asOf && marketCap > best.marketCap) {
			best = companySnapshot{
				name:      name,
				asOf:      asOf,
				marketCap: marketCap,
			}
		}
	}

	if best.asOf == "" {
		return companySnapshot{}, fmt.Errorf("no valid rows")
	}
	return best, nil
}

func getCSVCell(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func parseMarketCap(raw string) int64 {
	clean := strings.ReplaceAll(strings.TrimSpace(raw), ",", "")
	if clean == "" {
		return 0
	}

	v, err := strconv.ParseInt(clean, 10, 64)
	if err == nil {
		return v
	}
	f, ferr := strconv.ParseFloat(clean, 64)
	if ferr != nil {
		return 0
	}
	return int64(f)
}
