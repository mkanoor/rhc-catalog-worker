package request

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/RedHatInsights/rhc-worker-catalog/internal/catalogtask"
	"github.com/RedHatInsights/rhc-worker-catalog/internal/common"
	"github.com/RedHatInsights/rhc-worker-catalog/internal/logger"
	"github.com/RedHatInsights/rhc-worker-catalog/internal/towerapiworker"
)

type fakeHandler struct {
	timesCalled uint32
}

func (fh *fakeHandler) StartWork(ctx context.Context, config *common.CatalogConfig, params common.JobParam, client *http.Client, wc towerapiworker.WorkChannels) error {
	atomic.AddUint32(&fh.timesCalled, 1)
	return nil
}

type fakeCatalogTask struct{}

func (task *fakeCatalogTask) Get() (*common.CatalogInventoryTask, error) {
	message := common.CatalogInventoryTask{
		ID:     "12345",
		State:  "pending",
		Status: "ok",
		Input: common.RequestInput{
			ResponseFormat: "tar",
			Jobs: []common.JobParam{
				{Method: "monitor", HrefSlug: "/api/v2/jobs/7008"},
				{Method: "get", HrefSlug: "/api/v2/inventories/899"},
			},
		},
	}
	return &message, nil
}

func (task *fakeCatalogTask) Update(data map[string]interface{}) error {
	if data["state"] != "running" {
		return fmt.Errorf("Expected to receive running state, actual: %v", data["state"])
	}
	if data["message"] == nil {
		return fmt.Errorf("Expected message not to be empty")
	}
	return nil
}

type fakePageWriter struct{}

func (pw *fakePageWriter) Write(name string, b []byte) error { return nil }
func (pw *fakePageWriter) Flush() error                      { return nil }
func (pw *fakePageWriter) FlushErrors(msg []string) error    { return nil }

type fakePageWriterFactory struct{}

func (factory *fakePageWriterFactory) makePageWriter(ctx context.Context, input common.RequestInput, task catalogtask.CatalogTask, metadata map[string]string) (common.PageWriter, error) {
	return &fakePageWriter{}, nil
}

func TestProcessRequest(t *testing.T) {
	fh := fakeHandler{}
	ct := fakeCatalogTask{}
	pwf := fakePageWriterFactory{}
	shutdown := make(chan struct{})
	processRequest(logger.CtxWithLoggerID(context.Background(), "123"), "testurl", &common.CatalogConfig{}, &fh, &ct, &pwf, shutdown)
	if fh.timesCalled != 2 {
		t.Fatalf("2 workers should have been started only %d were started", fh.timesCalled)
	}
}

func TestMakePageWriter(t *testing.T) {
	ctx := logger.CtxWithLoggerID(context.Background(), "123")
	factory := defaultPageWriterFactory{}
	metadata := map[string]string{"task_url": "testurl"}

	pw, _ := factory.makePageWriter(ctx, common.RequestInput{ResponseFormat: "tar"}, catalogtask.MakeCatalogTask(ctx, "testurl"), metadata)
	pwType := fmt.Sprintf("%v", reflect.TypeOf(pw))
	assert.Equal(t, "*tarwriter.tarWriter", pwType, "Page Writer Type")

	pw, _ = factory.makePageWriter(ctx, common.RequestInput{ResponseFormat: "json"}, catalogtask.MakeCatalogTask(ctx, "testurl"), metadata)
	pwType = fmt.Sprintf("%v", reflect.TypeOf(pw))
	assert.Equal(t, "*jsonwriter.jsonWriter", pwType, "Page Writer Type")

	_, err := factory.makePageWriter(ctx, common.RequestInput{ResponseFormat: "gzip"}, catalogtask.MakeCatalogTask(ctx, "testurl"), metadata)
	assert.Error(t, err, "makePageWriter")
}
