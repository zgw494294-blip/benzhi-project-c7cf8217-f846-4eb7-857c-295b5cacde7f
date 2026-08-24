package domain

import "time"

type VolumeState string

const (
	StateDraft           VolumeState = "Draft"
	StateTranscribing    VolumeState = "Transcribing"
	StateChecking        VolumeState = "Checking"
	StateNeedsCorrection VolumeState = "NeedsCorrection"
	StateReadyForReview  VolumeState = "ReadyForReview"
	StateFrozen          VolumeState = "Frozen"
	StateAccessioned     VolumeState = "Accessioned"
)

type FindingStatus string

const (
	FindingOpen     FindingStatus = "Open"
	FindingResolved FindingStatus = "Resolved"
)

type FindingCategory string

const (
	CategoryMissingGlyph FindingCategory = "MissingGlyph"
	CategoryVariant      FindingCategory = "Variant"
	CategoryLayoutBreak  FindingCategory = "LayoutBreak"
	CategoryFolio        FindingCategory = "FolioAnomaly"
)

type ViolationSeverity string

const (
	SeverityBlocker ViolationSeverity = "Blocker"
	SeverityWarning ViolationSeverity = "Warning"
)

type DigitizationVolume struct {
	ID            string              `json:"id"`
	Title         string              `json:"title"`
	EditionNote   string              `json:"editionNote"`
	ShelfMark     string              `json:"shelfMark"`
	State         VolumeState         `json:"state"`
	Version       int64               `json:"version"`
	PageOrder     []string            `json:"pageOrder"`
	LatestCheckID string              `json:"latestCheckID,omitempty"`
	FrozenDigest  string              `json:"frozenDigest,omitempty"`
	CreatedAt     time.Time           `json:"createdAt"`
	UpdatedAt     time.Time           `json:"updatedAt"`
	Pages         []FacsimilePage     `json:"pages"`
	Findings      []CollationFinding  `json:"findings"`
	Checks        []IntegrityCheckRun `json:"checks"`
	Manifest      *AccessionManifest  `json:"manifest,omitempty"`
}

type FacsimilePage struct {
	ID             string `json:"id"`
	VolumeID       string `json:"volumeID"`
	FolioLabel     string `json:"folioLabel"`
	Sequence       int    `json:"sequence"`
	ImageObjectKey string `json:"imageObjectKey"`
	MediaType      string `json:"mediaType"`
	ByteSize       int64  `json:"byteSize"`
	SHA256         string `json:"sha256"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	Transcription  string `json:"transcription"`
	Revision       int64  `json:"revision"`
}

type CollationFinding struct {
	ID           string          `json:"id"`
	PageID       string          `json:"pageID"`
	Location     string          `json:"location"`
	Category     FindingCategory `json:"category"`
	ObservedText string          `json:"observedText"`
	ProposedText string          `json:"proposedText"`
	Status       FindingStatus   `json:"status"`
	Resolution   string          `json:"resolution,omitempty"`
	ResolvedBy   string          `json:"resolvedBy,omitempty"`
	ResolvedAt   *time.Time      `json:"resolvedAt,omitempty"`
}

type IntegrityViolation struct {
	Code       string            `json:"code"`
	Severity   ViolationSeverity `json:"severity"`
	PageID     string            `json:"pageID,omitempty"`
	FolioLabel string            `json:"folioLabel,omitempty"`
	Message    string            `json:"message"`
}

type IntegrityCheckRun struct {
	ID             string               `json:"id"`
	VolumeID       string               `json:"volumeID"`
	RunNumber      int                  `json:"runNumber"`
	Status         string               `json:"status"`
	CheckedVersion int64                `json:"checkedVersion"`
	Violations     []IntegrityViolation `json:"violations"`
	StartedAt      time.Time            `json:"startedAt"`
	CompletedAt    time.Time            `json:"completedAt"`
}

type PageDigest struct {
	PageID        string `json:"pageID"`
	FolioLabel    string `json:"folioLabel"`
	Sequence      int    `json:"sequence"`
	ImageSHA256   string `json:"imageSHA256"`
	Transcription string `json:"transcriptionSHA256"`
	Combined      string `json:"combinedSHA256"`
}

type AccessionManifest struct {
	ID             string       `json:"id"`
	VolumeID       string       `json:"volumeID"`
	ManifestNumber string       `json:"manifestNumber"`
	FrozenDigest   string       `json:"frozenDigest"`
	PageDigests    []PageDigest `json:"pageDigests"`
	Reviewer       string       `json:"reviewer"`
	ReviewNote     string       `json:"reviewNote"`
	IssuedAt       time.Time    `json:"issuedAt"`
}
