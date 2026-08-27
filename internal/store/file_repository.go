package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"trapreview/internal/application"
	"trapreview/internal/domain"
)

type FileRepository struct {
	mu           sync.RWMutex
	surveyLocks  map[string]*sync.Mutex
	state        snapshot
	eventsPath   string
	snapshotPath string
}

func Open(dataDirectory string) (*FileRepository, error) {
	if dataDirectory == "" {
		return nil, domain.Required("dataDir")
	}
	if err := os.MkdirAll(dataDirectory, 0o700); err != nil {
		return nil, fmt.Errorf("创建数据目录: %w", err)
	}
	eventsPath := filepath.Join(dataDirectory, "events.jsonl")
	snapshotPath := filepath.Join(dataDirectory, "snapshot.json")
	state, err := recoverState(eventsPath, snapshotPath)
	if err != nil {
		return nil, err
	}
	repository := &FileRepository{state: state, surveyLocks: map[string]*sync.Mutex{}, eventsPath: eventsPath, snapshotPath: snapshotPath}
	if state.LastSequence > 0 {
		if err := writeSnapshot(snapshotPath, state); err != nil {
			return nil, err
		}
	}
	return repository, nil
}

func (r *FileRepository) lockFor(surveyID string) *sync.Mutex {
	r.mu.Lock()
	defer r.mu.Unlock()
	lock, ok := r.surveyLocks[surveyID]
	if !ok {
		lock = &sync.Mutex{}
		r.surveyLocks[surveyID] = lock
	}
	return lock
}

func (r *FileRepository) Create(ctx context.Context, aggregate *domain.Aggregate, key string, result json.RawMessage) (json.RawMessage, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	lock := r.lockFor(aggregate.Survey.ID)
	lock.Lock()
	defer lock.Unlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	ref := idempotencyRef("create", key)
	if previous, ok := r.state.Idempotency[ref]; ok {
		return cloneRaw(previous), true, nil
	}
	if _, ok := r.state.Surveys[aggregate.Survey.ID]; ok {
		return nil, false, domain.NewError(domain.CodeConflict, "调查标识已存在", "id")
	}
	copy, err := cloneAggregate(aggregate)
	if err != nil {
		return nil, false, err
	}
	if err := r.commitLocked("survey_created", copy, ref, result); err != nil {
		return nil, false, err
	}
	return cloneRaw(result), false, nil
}

func (r *FileRepository) Transact(ctx context.Context, surveyID string, expected int64, key string, mutate application.Mutation) (json.RawMessage, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	lock := r.lockFor(surveyID)
	lock.Lock()
	defer lock.Unlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	ref := idempotencyRef(surveyID, key)
	if previous, ok := r.state.Idempotency[ref]; ok {
		return cloneRaw(previous), true, nil
	}
	current, ok := r.state.Surveys[surveyID]
	if !ok {
		return nil, false, domain.NewError(domain.CodeNotFound, "调查任务不存在", "surveyId")
	}
	if expected <= 0 {
		return nil, false, domain.Required("expectedVersion")
	}
	if current.Survey.ExpectedVersion != expected {
		return nil, false, domain.NewError(domain.CodeConflict, fmt.Sprintf("版本冲突：当前版本为 %d", current.Survey.ExpectedVersion), "expectedVersion")
	}
	working, err := cloneAggregate(current)
	if err != nil {
		return nil, false, err
	}
	result, err := mutate(working)
	if err != nil {
		return nil, false, err
	}
	if working.Survey.ExpectedVersion <= current.Survey.ExpectedVersion {
		return nil, false, fmt.Errorf("事务未推进调查版本")
	}
	if err := r.commitLocked("survey_updated", working, ref, result); err != nil {
		return nil, false, err
	}
	return cloneRaw(result), false, nil
}

func (r *FileRepository) commitLocked(kind string, aggregate *domain.Aggregate, ref string, result json.RawMessage) error {
	event := eventRecord{SchemaVersion: schemaVersion, Sequence: r.state.LastSequence + 1, Kind: kind, SurveyID: aggregate.Survey.ID, SurveyVersion: aggregate.Survey.ExpectedVersion, IdempotencyRef: ref, Result: cloneRaw(result), Aggregate: aggregate, RecordedAt: time.Now().UTC()}
	checksum, err := calculateChecksum(event)
	if err != nil {
		return err
	}
	event.Checksum = checksum
	r.state.LastSequence = event.Sequence
	r.state.Surveys[aggregate.Survey.ID] = aggregate
	r.state.Idempotency[ref] = cloneRaw(result)
	for _, release := range aggregate.Releases {
		r.state.Releases[release.VerificationCode] = release
	}
	if err := appendEvent(r.eventsPath, event); err != nil {
		return err
	}
	return writeSnapshot(r.snapshotPath, r.state)
}

func (r *FileRepository) Load(ctx context.Context, surveyID string) (*domain.Aggregate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	aggregate, ok := r.state.Surveys[surveyID]
	if !ok {
		return nil, domain.NewError(domain.CodeNotFound, "调查任务不存在", "surveyId")
	}
	return cloneAggregate(aggregate)
}

func (r *FileRepository) LookupIdempotency(ctx context.Context, scope, key string) (json.RawMessage, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result, ok := r.state.Idempotency[idempotencyRef(scope, key)]
	return cloneRaw(result), ok, nil
}

func (r *FileRepository) FindRelease(ctx context.Context, code string) (*domain.DatasetRelease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	release, ok := r.state.Releases[code]
	if !ok {
		return nil, domain.NewError(domain.CodeNotFound, "发布凭据不存在", "verificationCode")
	}
	if release.VerificationCode != code {
		return nil, fmt.Errorf("发布凭据索引不一致")
	}
	if err := release.ValidateManifest(); err != nil {
		return nil, fmt.Errorf("发布内容完整性校验失败: %w", err)
	}
	copy := release
	copy.SpeciesCounts = make(map[string]int, len(release.SpeciesCounts))
	for species, count := range release.SpeciesCounts {
		copy.SpeciesCounts[species] = count
	}
	copy.Manifest.Items = append([]domain.ReleaseManifestItem(nil), release.Manifest.Items...)
	return &copy, nil
}
