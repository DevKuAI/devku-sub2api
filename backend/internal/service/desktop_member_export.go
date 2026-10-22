package service

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

var ErrDesktopExportEmpty = errors.New("desktop export contains no members")

type DesktopMemberExportFormat struct {
	Headers     []string          `json:"headers"`
	Active      string            `json:"active"`
	Disabled    string            `json:"disabled"`
	TokenLabels map[string]string `json:"token_labels"`
}

type DesktopMemberUsageExporter interface {
	StreamMembersWithUsage(context.Context, string, DesktopMemberListFilters, time.Time, func(*DesktopOrganization, []DesktopMember) error) error
}

func safeDesktopCSV(value string) string {
	trimmed := strings.TrimLeftFunc(value, func(r rune) bool { return unicode.IsSpace(r) || r == '\uFEFF' })
	if (len(trimmed) > 0 && strings.ContainsRune("=+-@", rune(trimmed[0]))) || strings.HasPrefix(value, "\t") || strings.HasPrefix(value, "\r") || strings.HasPrefix(value, "\n") {
		return "'" + value
	}
	return value
}

func (s *DesktopService) ExportMemberUsage(ctx context.Context, org string, managerID int64, filters DesktopMemberListFilters, format DesktopMemberExportFormat, output io.Writer) error {
	if len(format.Headers) != 19 {
		return ErrDesktopValidation
	}
	labels := append(append([]string{}, format.Headers...), format.Active, format.Disabled, format.TokenLabels["active"], format.TokenLabels["disabled"], format.TokenLabels["missing"])
	for _, label := range labels {
		if !utf8.ValidString(label) || utf8.RuneCountInString(label) < 1 || utf8.RuneCountInString(label) > 120 || strings.ContainsRune(label, 0) {
			return ErrDesktopValidation
		}
	}
	repo := s.repo
	if managerID > 0 {
		managed, organization, err := s.managedService(ctx, managerID)
		if err != nil {
			return err
		}
		repo = managed.repo
		org = organization.PublicID
	}
	exporter, ok := repo.(DesktopMemberUsageExporter)
	if !ok {
		return ErrDesktopUsageUnavailable
	}
	// Reuse the normal search normalization and validation before opening the snapshot.
	filters.Search = strings.TrimSpace(filters.Search)
	if filters.Status != "" && filters.Status != DesktopStatusActive && filters.Status != DesktopStatusDisabled {
		return ErrDesktopValidation
	}
	if len([]rune(filters.Search)) > 100 {
		return ErrDesktopValidation
	}
	if phone, err := NormalizeDesktopPhone(filters.Search); err == nil {
		filters.Phone = phone
		filters.Search = ""
	}
	if _, err := io.WriteString(output, "\uFEFF"); err != nil {
		return err
	}
	writer := csv.NewWriter(output)
	writer.UseCRLF = true
	headers := append([]string{}, format.Headers...)
	for i, value := range headers {
		headers[i] = safeDesktopCSV(value)
	}
	if err := writer.Write(headers); err != nil {
		return err
	}
	asOf := s.now().UTC()
	count := 0
	tokenText := func(value int64, status string) string {
		if status == "unavailable" {
			return ""
		}
		return fmt.Sprint(value)
	}
	err := exporter.StreamMembersWithUsage(ctx, org, filters, asOf, func(organization *DesktopOrganization, members []DesktopMember) error {
		for _, member := range members {
			count++
			if ctx.Err() != nil {
				return ctx.Err()
			}
			status := format.Active
			if member.Status != DesktopStatusActive {
				status = format.Disabled
			}
			department, position := "", ""
			if member.Department != nil {
				department = member.Department.Name
			}
			if member.Position != nil {
				position = member.Position.Name
			}
			usage := member.Usage
			if usage == nil {
				return ErrDesktopUsageUnavailable
			}
			row := []string{organization.Name, organization.Code, member.PublicID, member.Name, member.Remark, department, position, member.Phone, status, format.TokenLabels[member.ModelTokenStatus()], fmt.Sprint(usage.TodayActualCost), fmt.Sprint(usage.Last30DaysActualCost), fmt.Sprint(usage.TotalActualCost), tokenText(usage.TodayTokens, usage.TodayTokenStatus), tokenText(usage.Last30DaysTokens, usage.Last30DaysTokenStatus), tokenText(usage.TotalTokens, usage.TotalTokenStatus), member.UpdatedAt.UTC().Format(time.RFC3339Nano), asOf.Format(time.RFC3339Nano), timezone.Location().String()}
			for i, value := range row {
				if i < 10 || i > 15 {
					row[i] = safeDesktopCSV(value)
				}
			}
			if err := writer.Write(row); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	})
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrDesktopExportEmpty
	}
	writer.Flush()
	return writer.Error()
}
