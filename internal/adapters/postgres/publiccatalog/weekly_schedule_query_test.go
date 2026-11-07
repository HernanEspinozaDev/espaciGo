package publiccatalogpg

import (
	"strings"
	"testing"
)

// This is query-text boundary coverage, not PostgreSQL integration coverage.
func TestDetailWeeklyScheduleSubqueryPrecedesEligibilityWhere(t *testing.T) {
	query := detailQuery()
	where := strings.Index(query, "WHERE e.estado='activa'")
	if where < 0 {
		t.Fatal("eligibility WHERE not found")
	}
	if strings.Index(query, "LEFT JOIN public.espacio_horario_semanal") >= 0 {
		t.Fatal("schedule JOIN must not be appended after eligibility WHERE")
	}
	if !strings.Contains(query[:where], "FROM public.espacio_horario_semanal h WHERE h.espacio_id=e.id") {
		t.Fatal("missing correlated schedule subquery before eligibility WHERE")
	}
	if !strings.HasSuffix(strings.TrimSpace(query), "AND e.id=$1") {
		t.Fatal("detail id filter must remain after eligiblePublicSQL")
	}
}
