package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Authorized          map[string]struct{}
	PairingPhone        string
	Timezone            *time.Location
	DataDir             string
	DNSServer           string
	PairingQRPath       string
	ReportRetentionDays int
	BackupRetentionDays int
	LogRetentionDays    int
	DailySummaryHour    int
	BackupHour          int
}

func Load() (Config, error) {
	zone := env("TIMEZONE", "America/Bahia")
	location, err := time.LoadLocation(zone)
	if err != nil {
		return Config{}, fmt.Errorf("timezone %s: %w", zone, err)
	}
	c := Config{Authorized: map[string]struct{}{}, PairingPhone: digits(os.Getenv("PAIRING_PHONE")), Timezone: location, DataDir: env("DATA_DIR", "./data")}
	c.DNSServer = strings.TrimSpace(os.Getenv("DNS_SERVER"))
	c.PairingQRPath = strings.TrimSpace(os.Getenv("PAIRING_QR_PATH"))
	for _, raw := range strings.Split(os.Getenv("AUTHORIZED_NUMBERS"), ",") {
		if n := digits(raw); n != "" {
			c.Authorized[n] = struct{}{}
		}
	}
	if c.ReportRetentionDays, err = parsePositive("REPORT_RETENTION_DAYS", 7); err != nil {
		return Config{}, err
	}
	if c.BackupRetentionDays, err = parsePositive("BACKUP_RETENTION_DAYS", 30); err != nil {
		return Config{}, err
	}
	if c.LogRetentionDays, err = parsePositive("LOG_RETENTION_DAYS", 14); err != nil {
		return Config{}, err
	}
	if c.DailySummaryHour, err = parseHour("DAILY_SUMMARY_HOUR", 17); err != nil {
		return Config{}, err
	}
	if c.BackupHour, err = parseHour("BACKUP_HOUR", 16); err != nil {
		return Config{}, err
	}
	c.DataDir = filepath.Clean(c.DataDir)
	return c, nil
}
func parseHour(k string, d int) (int, error) {
	v := env(k, strconv.Itoa(d))
	n, e := strconv.Atoi(v)
	if e != nil || n < 0 || n > 23 {
		return 0, fmt.Errorf("%s must be between 0 and 23", k)
	}
	return n, nil
}

func (c Config) IsAuthorized(number string) bool {
	for _, candidate := range brazilianNumberVariants(digits(number)) {
		if _, ok := c.Authorized[candidate]; ok {
			return true
		}
	}
	return false
}
func SamePhoneNumber(a, b string) bool {
	for _, left := range brazilianNumberVariants(digits(a)) {
		for _, right := range brazilianNumberVariants(digits(b)) {
			if left == right {
				return true
			}
		}
	}
	return false
}
func brazilianNumberVariants(number string) []string {
	result := []string{number}
	if len(number) == 13 && strings.HasPrefix(number, "55") && number[4] == '9' {
		result = append(result, number[:4]+number[5:])
	} else if len(number) == 12 && strings.HasPrefix(number, "55") {
		result = append(result, number[:4]+"9"+number[4:])
	}
	return result
}
func digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
func env(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}
func parsePositive(k string, d int) (int, error) {
	v := env(k, strconv.Itoa(d))
	n, e := strconv.Atoi(v)
	if e != nil || n < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", k)
	}
	return n, nil
}
