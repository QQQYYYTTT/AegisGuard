package attacks

import (
	"sync"

	"aegisguard/backend/internal/catalog"
)

var (
	libraryOnce sync.Once
	libraryData Library
)

func GetLibrary() Library {
	libraryOnce.Do(func() {
		libraryData = buildLibrary()
	})
	return libraryData
}

func buildLibrary() Library {
	families := catalog.AttackFamilies
	grouped := make([]FamilyBundle, 0, len(families))
	for _, family := range families {
		grouped = append(grouped, FamilyBundle{
			FamilyID:   family.ID,
			FamilyName: family.Name,
			Cases:      nil,
		})
	}

	return Library{
		Overview:      "Local JSON attack fixtures have been removed. AegisGuard now runs the original ASB benchmark through experiments/asb and records converted outputs in the shared evaluation schema.",
		Families:      families,
		Cases:         nil,
		CasesByFamily: grouped,
	}
}
