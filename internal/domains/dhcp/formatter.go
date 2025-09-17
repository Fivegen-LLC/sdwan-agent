package dhcp

import (
	"bufio"
	"fmt"
	"net"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
)

const (
	ipColumnIndex = 2
)

var (
	searchKeys   map[string]bool
	headerKeys   = table.Row{"#", "MAC", "IP", "HOSTNAME", "BEGIN", "END", "MANUFACTURER"}
	skipPrefixes = []string{
		"To get manufacturer",
	}
)

func init() {
	searchKeys = lo.SliceToMap(headerKeys, func(item any) (string, bool) {
		return fmt.Sprintf("%v", item), true
	})
}

// formatDHCPLeaseToTable formats dhcp lease output to pretty table.
func formatDHCPLeaseToTable(input string, filterLinkIP *string) (output string, err error) {
	t := table.NewWriter()
	t.AppendHeader(headerKeys)

	var (
		rowNumber = 1
		scanner   = bufio.NewScanner(strings.NewReader(input))
	)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if lo.IsEmpty(line) {
			continue
		}

		var skipLine bool
		for _, skipPrefix := range skipPrefixes {
			if strings.HasPrefix(line, skipPrefix) {
				skipLine = true
				break
			}
		}

		if skipLine {
			continue
		}

		var row []any
		row = append(row, rowNumber)

		var (
			buf    strings.Builder
			values = strings.Split(line, " ")
		)
		for _, value := range values {
			if !searchKeys[value] {
				if buf.Len() > 0 {
					buf.WriteString(" ")
				}
				buf.WriteString(value)
				continue
			}

			if buf.Len() > 0 {
				row = append(row, buf.String())
				buf.Reset()
			}
		}

		if buf.Len() > 0 {
			row = append(row, buf.String())
		}

		// check lease belongs to link
		skip, skipErr := skipRow(row[ipColumnIndex], filterLinkIP)
		if skipErr != nil {
			log.Error().Err(skipErr).Msg("formatDHCPLeaseToTable: skip row error")
		}

		if !skip {
			t.AppendRow(row)
			rowNumber++
		}
	}

	if err = scanner.Err(); err != nil {
		return output, fmt.Errorf("formatDHCPLeaseToTable: %w", err)
	}

	return t.Render(), nil
}

func skipRow(ipValue any, filterLinkIP *string) (skip bool, err error) {
	if filterLinkIP == nil {
		return false, nil
	}

	_, linkNet, err := net.ParseCIDR(*filterLinkIP)
	if err != nil {
		return skip, fmt.Errorf("skipRow: %w", err)
	}

	ipStr := strings.TrimSpace(fmt.Sprintf("%v", ipValue))
	ip := net.ParseIP(ipStr)
	if ip.To4() == nil {
		return skip, fmt.Errorf("skipRow: invalid ip address %v", ipStr)
	}

	return !linkNet.Contains(ip), nil
}
