package postgres

import (
	"errors"
	"fmt"
)

const (
	GCP04ReplayPoolCount              int32 = 3
	GCP04ReplayOperationalConnReserve int32 = 5
	GCP04ReplayConnectionDemand       int32 = GCP04ReplayPoolCount*GCP04ReplayPoolMaxConns + GCP04ReplayOperationalConnReserve
)

type GCP04ReplayCapacityFacts struct {
	Database              string
	CurrentUser           string
	ServerVersionNum      int
	MaxConnections        int32
	SuperuserReserved     int32
	ReservedConnections   int32
	OtherConnections      int32
	RoleCanLogin          bool
	RoleCanCreateDatabase bool
	RoleIsSuperuser       bool
	InRecovery            bool
}

type GCP04ReplayCapacityAssessment struct {
	Demand                 int32
	OrdinaryCapacity       int32
	OtherConnections       int32
	AvailableOrdinarySlots int32
	MaxConnections         int32
	SuperuserReserved      int32
	ReservedConnections    int32
	ServerVersionNum       int
	CurrentUser            string
}

// AssessGCP04ReplayCapacity validates a read-only snapshot before the replay
// harness creates its synthetic database or applies migrations. The capacity
// check is intentionally conservative: every other current connection is
// counted against ordinary slots, even if it could use a reserved slot.
func AssessGCP04ReplayCapacity(facts GCP04ReplayCapacityFacts) (GCP04ReplayCapacityAssessment, error) {
	assessment := GCP04ReplayCapacityAssessment{
		Demand: GCP04ReplayConnectionDemand, MaxConnections: facts.MaxConnections,
		SuperuserReserved: facts.SuperuserReserved, ReservedConnections: facts.ReservedConnections,
		OtherConnections: facts.OtherConnections, ServerVersionNum: facts.ServerVersionNum,
		CurrentUser: facts.CurrentUser,
	}
	if facts.Database != "postgres" || facts.CurrentUser != "postgres" {
		return assessment, errors.New("GCP-04 preflight must connect to the postgres database as postgres")
	}
	if facts.ServerVersionNum/10000 != 18 || facts.InRecovery {
		return assessment, errors.New("GCP-04 preflight requires a writable PostgreSQL 18 primary")
	}
	if !facts.RoleCanLogin || (!facts.RoleCanCreateDatabase && !facts.RoleIsSuperuser) {
		return assessment, errors.New("GCP-04 preflight role cannot log in and create the isolated test database")
	}
	if facts.MaxConnections <= 0 || facts.SuperuserReserved < 0 || facts.ReservedConnections < 0 || facts.OtherConnections < 0 {
		return assessment, errors.New("GCP-04 preflight returned invalid connection limits")
	}
	reserved := int64(facts.SuperuserReserved) + int64(facts.ReservedConnections)
	if reserved >= int64(facts.MaxConnections) || facts.OtherConnections > facts.MaxConnections {
		return assessment, errors.New("GCP-04 preflight reserved or active connection counts exceed max_connections")
	}
	ordinaryCapacity := int64(facts.MaxConnections) - reserved
	available := ordinaryCapacity - int64(facts.OtherConnections)
	assessment.OrdinaryCapacity = int32(ordinaryCapacity)
	assessment.AvailableOrdinarySlots = int32(available)
	if assessment.AvailableOrdinarySlots < assessment.Demand {
		return assessment, fmt.Errorf("GCP-04 preflight needs %d ordinary slots after reservations; only %d are available", assessment.Demand, assessment.AvailableOrdinarySlots)
	}
	return assessment, nil
}
