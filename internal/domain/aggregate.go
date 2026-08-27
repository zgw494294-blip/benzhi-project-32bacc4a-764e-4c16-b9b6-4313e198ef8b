package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type Aggregate struct {
	Survey          Survey                   `json:"survey"`
	Stations        map[string]CameraStation `json:"stations"`
	Observations    map[string]Observation   `json:"observations"`
	Adjudications   []Adjudication           `json:"adjudications"`
	Releases        []DatasetRelease         `json:"releases"`
	AuditTrail      []AuditEntry             `json:"auditTrail"`
	ReviewRequested bool                     `json:"reviewRequested"`
	ApprovedBy      string                   `json:"approvedBy,omitempty"`
	ApprovedAt      *time.Time               `json:"approvedAt,omitempty"`
}

func NewAggregate(s Survey) *Aggregate {
	a := &Aggregate{Survey: s, Stations: map[string]CameraStation{}, Observations: map[string]Observation{}, Adjudications: []Adjudication{}, Releases: []DatasetRelease{}, AuditTrail: []AuditEntry{}}
	a.record("survey_created", s.LeadResearcher, s.Title, s.CreatedAt)
	return a
}

func (a *Aggregate) ReviseSurvey(title, lead string, catalog []string, actor string, now time.Time) error {
	if a.Survey.Status != SurveyDraft {
		return InvalidState(a.Survey.Status, "修订调查方案")
	}
	if err := a.Survey.Revise(title, lead, catalog); err != nil {
		return err
	}
	a.touch("survey_revised", actor, fmt.Sprintf("物种目录 %d 项", len(a.Survey.SpeciesCatalog)), now)
	return nil
}

func (a *Aggregate) AddStation(station CameraStation, actor string, now time.Time) error {
	if a.Survey.Status != SurveyDraft {
		return InvalidState(a.Survey.Status, "登记点位")
	}
	for _, existing := range a.Stations {
		if existing.StationCode == station.StationCode {
			return Invalid("stationCode", "在本调查中已存在")
		}
	}
	a.Stations[station.ID] = station
	a.touch("station_added", actor, station.StationCode, now)
	return nil
}

func (a *Aggregate) UpdateStation(station CameraStation, actor string, now time.Time) error {
	if a.Survey.Status != SurveyDraft {
		return InvalidState(a.Survey.Status, "更新点位")
	}
	if _, ok := a.Stations[station.ID]; !ok {
		return NewError(CodeNotFound, "相机点位不存在", "stationId")
	}
	for id, existing := range a.Stations {
		if id != station.ID && existing.StationCode == station.StationCode {
			return Invalid("stationCode", "在本调查中已存在")
		}
	}
	a.Stations[station.ID] = station
	a.touch("station_updated", actor, station.StationCode, now)
	return nil
}

func (a *Aggregate) RemoveStation(stationID, actor string, now time.Time) error {
	if a.Survey.Status != SurveyDraft {
		return InvalidState(a.Survey.Status, "撤销点位")
	}
	station, ok := a.Stations[stationID]
	if !ok {
		return NewError(CodeNotFound, "相机点位不存在", "stationId")
	}
	delete(a.Stations, stationID)
	a.touch("station_removed", actor, station.StationCode, now)
	return nil
}

func (a *Aggregate) LockProtocol(actor string, now time.Time) error {
	if a.Survey.Status != SurveyDraft {
		return InvalidState(a.Survey.Status, "锁定方案")
	}
	if len(a.Stations) == 0 {
		return NewError(CodeValidation, "至少登记一个相机点位才能锁定方案", "stations")
	}
	if len(a.Survey.SpeciesCatalog) == 0 {
		return Required("speciesCatalog")
	}
	a.Survey.Status = SurveyProtocolLocked
	a.touch("protocol_locked", actor, fmt.Sprintf("%d 个点位", len(a.Stations)), now)
	return nil
}

func (a *Aggregate) AddObservation(observation Observation, actor string, now time.Time) error {
	return a.AddObservations([]Observation{observation}, actor, now)
}

func (a *Aggregate) AddObservations(observations []Observation, actor string, now time.Time) error {
	if !a.Survey.Status.CanRecord() {
		return InvalidState(a.Survey.Status, "提交观测")
	}
	if len(observations) == 0 {
		return Required("items")
	}
	refs, checksums := make(map[string]struct{}, len(a.Observations)), make(map[string]struct{}, len(a.Observations))
	for _, existing := range a.Observations {
		if ref := strings.TrimSpace(existing.MediaRef); ref != "" {
			refs[ref] = struct{}{}
		}
		if checksum := strings.ToLower(strings.TrimSpace(existing.MediaChecksum)); checksum != "" {
			checksums[checksum] = struct{}{}
		}
	}
	for index, observation := range observations {
		station, ok := a.Stations[observation.StationID]
		if !ok {
			return NewError(CodeNotFound, fmt.Sprintf("第 %d 项的相机点位不存在", index), fmt.Sprintf("items[%d].stationId", index))
		}
		if !station.Contains(observation.CapturedAt) {
			return Invalid(fmt.Sprintf("items[%d].capturedAt", index), "不在点位布设时间范围内")
		}
		if _, exists := a.Observations[observation.ID]; exists {
			return Invalid(fmt.Sprintf("items[%d].id", index), "观测已存在")
		}
		ref := strings.TrimSpace(observation.MediaRef)
		if _, duplicate := refs[ref]; ref != "" && duplicate {
			return Invalid(fmt.Sprintf("items[%d].mediaRef", index), "证据引用已登记")
		}
		checksum := strings.ToLower(strings.TrimSpace(observation.MediaChecksum))
		if _, duplicate := checksums[checksum]; checksum != "" && duplicate {
			return Invalid(fmt.Sprintf("items[%d].mediaChecksum", index), "证据校验和已登记")
		}
		refs[ref], checksums[checksum] = struct{}{}, struct{}{}
	}
	for _, observation := range observations {
		a.Observations[observation.ID] = observation
	}
	a.Survey.Status = SurveyRecording
	a.touch("observations_submitted", actor, fmt.Sprintf("批次共 %d 条", len(observations)), now)
	return nil
}

func (a *Aggregate) SetVerification(id string, flags []string, actor string, now time.Time) error {
	observation, ok := a.Observations[id]
	if !ok {
		return NewError(CodeNotFound, "观测不存在", "observationId")
	}
	observation.ApplyVerification(flags, now)
	a.Observations[id] = observation
	a.touch("observation_verified", actor, fmt.Sprintf("%s:%s", id, observation.Status), now)
	return nil
}

func (a *Aggregate) VerifyObservations(results map[string][]string, actor string, now time.Time) error {
	if !a.Survey.Status.CanRecord() {
		return InvalidState(a.Survey.Status, "核验观测")
	}
	if len(results) == 0 {
		return NewError(CodeValidation, "没有处于 submitted 状态的目标观测", "observationIds")
	}
	ids := make([]string, 0, len(results))
	for id := range results {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		observation, ok := a.Observations[id]
		if !ok {
			return NewError(CodeNotFound, "观测不存在", "observationId")
		}
		if observation.Status != ObservationSubmitted {
			return Invalid("observationIds", fmt.Sprintf("观测 %s 不处于 submitted 状态", id))
		}
	}
	for _, id := range ids {
		observation := a.Observations[id]
		observation.ApplyVerification(results[id], now)
		a.Observations[id] = observation
	}
	if a.pendingAdjudicationCount() > 0 {
		a.Survey.Status = SurveyAdjudicating
	} else {
		a.Survey.Status = SurveyRecording
	}
	a.touch("observations_verified", actor, fmt.Sprintf("定向核验 %d 条，未决 %d", len(ids), a.UnresolvedCount()), now)
	return nil
}

func (a *Aggregate) pendingAdjudicationCount() int {
	count := 0
	for _, observation := range a.Observations {
		if observation.Status == ObservationDisputed || observation.Status == ObservationCorrectionRequired {
			count++
		}
	}
	return count
}

func (a *Aggregate) FinishVerification(actor string, now time.Time) error {
	if len(a.Observations) == 0 {
		return NewError(CodeValidation, "没有可核验的观测", "observations")
	}
	if a.UnresolvedCount() > 0 {
		a.Survey.Status = SurveyAdjudicating
	} else {
		a.Survey.Status = SurveyRecording
	}
	a.touch("verification_completed", actor, fmt.Sprintf("未决 %d", a.UnresolvedCount()), now)
	return nil
}

func (a *Aggregate) Adjudicate(adjudication Adjudication, actor string, now time.Time) error {
	observation, ok := a.Observations[adjudication.ObservationID]
	if !ok {
		return NewError(CodeNotFound, "观测不存在", "observationId")
	}
	if observation.Status != ObservationDisputed {
		return Invalid("observationId", "不处于待裁定状态")
	}
	switch adjudication.Decision {
	case DecisionAcceptPrimary:
		observation.ResolvedLabel, observation.Status = observation.PrimaryLabel, ObservationVerified
	case DecisionAcceptSecondary:
		observation.ResolvedLabel, observation.Status = observation.SecondaryLabel, ObservationVerified
	case DecisionResolveLabel:
		if !a.Survey.HasSpecies(adjudication.ResolvedLabel) {
			return Invalid("resolvedLabel", "不在物种目录中")
		}
		observation.ResolvedLabel, observation.Status = adjudication.ResolvedLabel, ObservationVerified
	case DecisionRequireCorrection:
		observation.Status = ObservationCorrectionRequired
	}
	a.Observations[observation.ID] = observation
	a.Adjudications = append(a.Adjudications, adjudication)
	a.touch("observation_adjudicated", actor, string(adjudication.Decision), now)
	return nil
}

func (a *Aggregate) CorrectObservation(id, mediaRef, checksum, primary, secondary, actor string, now time.Time) error {
	observation, ok := a.Observations[id]
	if !ok {
		return NewError(CodeNotFound, "观测不存在", "observationId")
	}
	if observation.Status != ObservationCorrectionRequired {
		return Invalid("observationId", "没有定向修正要求")
	}
	observation.ApplyCorrection(mediaRef, checksum, primary, secondary)
	a.Observations[id] = observation
	a.touch("observation_corrected", actor, id, now)
	return nil
}

func (a *Aggregate) RequestReview(actor string, now time.Time) error {
	if a.Survey.Status != SurveyRecording && a.Survey.Status != SurveyAdjudicating {
		return InvalidState(a.Survey.Status, "申请复核")
	}
	if a.UnresolvedCount() > 0 {
		return NewError(CodeUnresolved, "仍有未解决的争议观测", "observations")
	}
	if len(a.Observations) == 0 {
		return NewError(CodeValidation, "至少需要一条观测", "observations")
	}
	a.ReviewRequested = true
	a.Survey.Status = SurveyReviewPending
	a.touch("review_requested", actor, "", now)
	return nil
}

func (a *Aggregate) Approve(reviewer string, now time.Time) error {
	if a.Survey.Status != SurveyReviewPending || !a.ReviewRequested {
		return InvalidState(a.Survey.Status, "批准复核")
	}
	if strings.TrimSpace(reviewer) == "" {
		return Required("reviewer")
	}
	if reviewer == a.Survey.LeadResearcher {
		return Invalid("reviewer", "必须独立于调查负责人")
	}
	if a.UnresolvedCount() > 0 {
		return NewError(CodeUnresolved, "仍有未解决的争议观测", "observations")
	}
	a.ApprovedBy = strings.TrimSpace(reviewer)
	approvedAt := now.UTC()
	a.ApprovedAt = &approvedAt
	a.Survey.Status = SurveyApproved
	a.touch("review_approved", reviewer, "", now)
	return nil
}

func (a *Aggregate) Publish(release DatasetRelease, actor string, now time.Time) error {
	if a.Survey.Status == SurveyPublished || len(a.Releases) > 0 {
		return NewError(CodeDuplicateRelease, "调查已经发布，发布版本不可变", "")
	}
	if a.Survey.Status != SurveyApproved {
		return InvalidState(a.Survey.Status, "发布数据集")
	}
	if a.UnresolvedCount() > 0 {
		return NewError(CodeUnresolved, "仍有未解决的争议观测", "observations")
	}
	a.Releases = append(a.Releases, release)
	a.Survey.Status = SurveyPublished
	a.touch("dataset_published", actor, release.VerificationCode, now)
	return nil
}

func (a *Aggregate) UnresolvedCount() int {
	count := 0
	for _, o := range a.Observations {
		if o.Status != ObservationVerified {
			count++
		}
	}
	return count
}

func (a *Aggregate) SortedObservations() []Observation {
	result := make([]Observation, 0, len(a.Observations))
	for _, o := range a.Observations {
		result = append(result, o)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (a *Aggregate) touch(action, actor, detail string, now time.Time) {
	a.Survey.ExpectedVersion++
	a.Survey.UpdatedAt = now.UTC()
	a.record(action, actor, detail, now)
}

func (a *Aggregate) record(action, actor, detail string, now time.Time) {
	a.AuditTrail = append(a.AuditTrail, AuditEntry{Sequence: int64(len(a.AuditTrail) + 1), Action: action, Actor: actor, Detail: detail, At: now.UTC()})
}
