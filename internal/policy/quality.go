package policy

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"trapreview/internal/domain"
)

const (
	FlagMissingMediaRef       = "missing_media_ref"
	FlagMissingMediaChecksum  = "missing_media_checksum"
	FlagInvalidMediaChecksum  = "invalid_media_checksum"
	FlagMissingPrimaryLabel   = "missing_primary_label"
	FlagMissingSecondaryLabel = "missing_secondary_label"
	FlagPrimaryOutOfCatalog   = "primary_out_of_catalog"
	FlagSecondaryOutOfCatalog = "secondary_out_of_catalog"
	FlagLabelConflict         = "label_conflict"
)

var checksumPattern = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

type QualityPolicy struct{}

func NewQualityPolicy() QualityPolicy { return QualityPolicy{} }

func (QualityPolicy) Evaluate(survey domain.Survey, observation domain.Observation) []string {
	flags := make([]string, 0, 5)
	if strings.TrimSpace(observation.MediaRef) == "" {
		flags = append(flags, FlagMissingMediaRef)
	}
	if strings.TrimSpace(observation.MediaChecksum) == "" {
		flags = append(flags, FlagMissingMediaChecksum)
	} else if !checksumPattern.MatchString(observation.MediaChecksum) {
		flags = append(flags, FlagInvalidMediaChecksum)
	}
	if observation.PrimaryLabel == "" {
		flags = append(flags, FlagMissingPrimaryLabel)
	} else if !survey.HasSpecies(observation.PrimaryLabel) {
		flags = append(flags, FlagPrimaryOutOfCatalog)
	}
	if observation.SecondaryLabel == "" {
		flags = append(flags, FlagMissingSecondaryLabel)
	} else if !survey.HasSpecies(observation.SecondaryLabel) {
		flags = append(flags, FlagSecondaryOutOfCatalog)
	}
	if observation.PrimaryLabel != "" && observation.SecondaryLabel != "" && observation.PrimaryLabel != observation.SecondaryLabel {
		flags = append(flags, FlagLabelConflict)
	}
	sort.Strings(flags)
	return flags
}

func (p QualityPolicy) ValidateSubmission(survey domain.Survey, observation domain.Observation, fieldPrefix string) error {
	for _, flag := range p.Evaluate(survey, observation) {
		field, message := "", ""
		switch flag {
		case FlagMissingMediaRef:
			field, message = "mediaRef", "不能为空"
		case FlagMissingMediaChecksum:
			field, message = "mediaChecksum", "不能为空"
		case FlagInvalidMediaChecksum:
			field, message = "mediaChecksum", "必须是 64 位 SHA-256 十六进制值"
		case FlagMissingPrimaryLabel:
			field, message = "primaryLabel", "不能为空"
		case FlagMissingSecondaryLabel:
			field, message = "secondaryLabel", "不能为空"
		case FlagPrimaryOutOfCatalog:
			field, message = "primaryLabel", "不在物种目录中"
		case FlagSecondaryOutOfCatalog:
			field, message = "secondaryLabel", "不在物种目录中"
		case FlagLabelConflict:
			continue
		}
		if fieldPrefix != "" {
			field = fmt.Sprintf("%s.%s", fieldPrefix, field)
		}
		return domain.Invalid(field, message)
	}
	return nil
}

func IsEvidenceFlag(flag string) bool {
	return flag == FlagMissingMediaRef || flag == FlagMissingMediaChecksum || flag == FlagInvalidMediaChecksum
}

func IsQualityFlag(flag string) bool {
	switch flag {
	case FlagMissingMediaRef, FlagMissingMediaChecksum, FlagInvalidMediaChecksum,
		FlagMissingPrimaryLabel, FlagMissingSecondaryLabel, FlagPrimaryOutOfCatalog,
		FlagSecondaryOutOfCatalog, FlagLabelConflict:
		return true
	default:
		return false
	}
}
