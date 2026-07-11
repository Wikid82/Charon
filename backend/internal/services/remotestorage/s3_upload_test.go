package remotestorage

import (
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// listBucketResultXML mirrors the subset of S3's ListBucketResult (list-type=2)
// response schema that s3Uploader.List needs to parse. Building the fake
// server's response via xml.Marshal (rather than manual string/Sprintf
// concatenation) keeps this fixture honest to the real XML content model and
// guarantees correct escaping of object keys/prefixes.
type listBucketResultXML struct {
	XMLName     xml.Name               `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListBucketResult"`
	Name        string                 `xml:"Name"`
	Prefix      string                 `xml:"Prefix"`
	IsTruncated bool                   `xml:"IsTruncated"`
	Contents    []listBucketContentXML `xml:"Contents"`
}

type listBucketContentXML struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
}

// fakeS3Server is a minimal S3-compatible HTTP server used to exercise
// s3Uploader's Upload/Delete/List/Test request/response handling end-to-end
// (spec §3.7) without depending on network access to a real S3-compatible
// endpoint. It deliberately does NOT verify SigV4 signatures — minio-go
// signs every request regardless of what the server does with that header,
// and the production code under test (s3.go) never inspects response
// signature-related state, only status codes and bodies — so a
// signature-blind fake is sufficient to exercise every branch in Upload,
// Delete, List, and Test.
type fakeS3Server struct {
	mu      sync.Mutex
	objects map[string][]byte // key -> body, path-style "/bucket/key"

	bucketExists     bool
	bucketExistsSC   int // override status code for HEAD /bucket; 0 = derive from bucketExists
	putStatusCode    int // override for PUT; 0 = success (200)
	deleteStatusCode int // override for DELETE; 0 = success (204)
	listStatusCode   int // override for GET (list); 0 = success (200)
}

func newFakeS3Server(bucketExists bool) *fakeS3Server {
	return &fakeS3Server{objects: make(map[string][]byte), bucketExists: bucketExists}
}

func (s *fakeS3Server) start(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(srv.Close)
	return srv
}

func (s *fakeS3Server) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)
	bucket := parts[0]
	var key string
	if len(parts) > 1 {
		key = parts[1]
	}

	switch r.Method {
	case http.MethodHead:
		s.handleHeadBucket(w, bucket)
	case http.MethodPut:
		s.handlePutObject(w, r, bucket, key)
	case http.MethodDelete:
		s.handleDeleteObject(w, bucket, key)
	case http.MethodGet:
		s.handleListObjects(w, r, bucket)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *fakeS3Server) handleHeadBucket(w http.ResponseWriter, _ string) {
	switch {
	case s.bucketExistsSC != 0:
		w.WriteHeader(s.bucketExistsSC)
	case s.bucketExists:
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (s *fakeS3Server) handlePutObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if s.putStatusCode != 0 {
		w.WriteHeader(s.putStatusCode)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.objects[bucket+"/"+key] = body
	w.Header().Set("ETag", `"fakeetag"`)
	w.WriteHeader(http.StatusOK)
}

func (s *fakeS3Server) handleDeleteObject(w http.ResponseWriter, bucket, key string) {
	if s.deleteStatusCode != 0 {
		w.WriteHeader(s.deleteStatusCode)
		return
	}
	delete(s.objects, bucket+"/"+key)
	w.WriteHeader(http.StatusNoContent)
}

func (s *fakeS3Server) handleListObjects(w http.ResponseWriter, r *http.Request, bucket string) {
	if r.URL.Query().Get("list-type") != "2" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if s.listStatusCode != 0 {
		w.WriteHeader(s.listStatusCode)
		return
	}

	prefix := r.URL.Query().Get("prefix")
	bucketPrefix := bucket + "/"

	result := listBucketResultXML{
		Name:        bucket,
		Prefix:      prefix,
		IsTruncated: false,
	}
	for fullKey, body := range s.objects {
		if !strings.HasPrefix(fullKey, bucketPrefix) {
			continue
		}
		key := strings.TrimPrefix(fullKey, bucketPrefix)
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}
		result.Contents = append(result.Contents, listBucketContentXML{
			Key:          key,
			LastModified: time.Now().UTC().Format(time.RFC3339),
			ETag:         `"x"`,
			Size:         int64(len(body)),
			StorageClass: "STANDARD",
		})
	}

	payload, err := xml.Marshal(result)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(payload) // nosemgrep: go.net.xss.no-direct-write-to-responsewriter-taint.no-direct-write-to-responsewriter-taint -- test-only fake S3 API server (httptest.Server) responding to the s3Uploader client under test with XML-encoded (xml.Marshal), not HTML; body is parsed by minio-go's XML decoder, never rendered in a browser, so this is not an XSS surface.
}

// newTestS3Uploader constructs a real s3Uploader (via the exported
// construction path, newS3Uploader) pointed at server's URL, with SSRF
// checks relaxed for the loopback fixture and path-style addressing forced
// (required since our fake server has no virtual-hosted-style routing).
func newTestS3Uploader(t *testing.T, server *httptest.Server, bucket string) Uploader {
	t.Helper()
	withPermissiveSSRFForLocalTest(t)

	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	uploader, err := newS3Uploader(S3Config{
		Endpoint:       u.Host,
		Region:         "us-east-1", // avoids a GetBucketLocation round trip our fake server doesn't implement
		Bucket:         bucket,
		UseSSL:         false,
		ForcePathStyle: true,
	}, S3Secrets{AccessKeyID: "test-access-key", SecretAccessKey: "test-secret-key"})
	require.NoError(t, err)
	return uploader
}

func TestS3Uploader_UploadListDeleteTest_Roundtrip(t *testing.T) {
	fake := newFakeS3Server(true)
	server := fake.start(t)
	uploader := newTestS3Uploader(t, server, "charon-backups")
	ctx := context.Background()

	require.NoError(t, uploader.Test(ctx), "Test() must confirm the bucket exists and write+remove its connectivity marker")

	localFile := filepath.Join(t.TempDir(), "backup.zip")
	require.NoError(t, os.WriteFile(localFile, []byte("fake s3 backup contents"), 0o600))

	require.NoError(t, uploader.Upload(ctx, localFile, "backups/backup_1.zip"))

	objects, err := uploader.List(ctx, "backups/")
	require.NoError(t, err)
	var names []string
	for _, o := range objects {
		names = append(names, o.Key)
		assert.Greater(t, o.Size, int64(0))
	}
	assert.Contains(t, names, "backups/backup_1.zip")

	require.NoError(t, uploader.Delete(ctx, "backups/backup_1.zip"))

	remaining, err := uploader.List(ctx, "backups/")
	require.NoError(t, err)
	assert.Empty(t, remaining)
}

func TestS3Uploader_Test_BucketDoesNotExist(t *testing.T) {
	fake := newFakeS3Server(false)
	server := fake.start(t)
	uploader := newTestS3Uploader(t, server, "missing-bucket")

	err := uploader.Test(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist or is not accessible")
}

func TestS3Uploader_Test_BucketExistsCheckTransportError(t *testing.T) {
	fake := newFakeS3Server(true)
	fake.bucketExistsSC = http.StatusInternalServerError
	server := fake.start(t)
	uploader := newTestS3Uploader(t, server, "charon-backups")

	err := uploader.Test(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "check bucket exists")
}

func TestS3Uploader_Test_MarkerWriteFailure(t *testing.T) {
	fake := newFakeS3Server(true)
	fake.putStatusCode = http.StatusInternalServerError
	server := fake.start(t)
	uploader := newTestS3Uploader(t, server, "charon-backups")

	err := uploader.Test(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write connection test marker")
}

func TestS3Uploader_Test_MarkerRemoveFailure(t *testing.T) {
	fake := newFakeS3Server(true)
	fake.deleteStatusCode = http.StatusInternalServerError
	server := fake.start(t)
	uploader := newTestS3Uploader(t, server, "charon-backups")

	err := uploader.Test(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remove connection test marker")
}

func TestS3Uploader_Upload_LocalFileMissing(t *testing.T) {
	fake := newFakeS3Server(true)
	server := fake.start(t)
	uploader := newTestS3Uploader(t, server, "charon-backups")

	err := uploader.Upload(context.Background(), filepath.Join(t.TempDir(), "does-not-exist.zip"), "backup.zip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open local file")
}

func TestS3Uploader_Upload_PutObjectFailure(t *testing.T) {
	fake := newFakeS3Server(true)
	fake.putStatusCode = http.StatusInternalServerError
	server := fake.start(t)
	uploader := newTestS3Uploader(t, server, "charon-backups")

	localFile := filepath.Join(t.TempDir(), "backup.zip")
	require.NoError(t, os.WriteFile(localFile, []byte("data"), 0o600))

	err := uploader.Upload(context.Background(), localFile, "backup.zip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "put object")
}

func TestS3Uploader_Delete_Failure(t *testing.T) {
	fake := newFakeS3Server(true)
	fake.deleteStatusCode = http.StatusInternalServerError
	server := fake.start(t)
	uploader := newTestS3Uploader(t, server, "charon-backups")

	err := uploader.Delete(context.Background(), "backup.zip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remove object")
}

func TestS3Uploader_List_Failure(t *testing.T) {
	fake := newFakeS3Server(true)
	fake.listStatusCode = http.StatusInternalServerError
	server := fake.start(t)
	uploader := newTestS3Uploader(t, server, "charon-backups")

	_, err := uploader.List(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list objects")
}

func TestS3Uploader_List_EmptyBucket(t *testing.T) {
	fake := newFakeS3Server(true)
	server := fake.start(t)
	uploader := newTestS3Uploader(t, server, "charon-backups")

	objects, err := uploader.List(context.Background(), "")
	require.NoError(t, err)
	assert.Empty(t, objects)
}

// --- newS3Uploader: construction-time validation ---

func TestNewS3Uploader_MissingEndpoint(t *testing.T) {
	_, err := newS3Uploader(S3Config{Bucket: "b"}, S3Secrets{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint is required")
}

func TestNewS3Uploader_MissingBucket(t *testing.T) {
	_, err := newS3Uploader(S3Config{Endpoint: "203.0.113.10"}, S3Secrets{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bucket is required")
}

func TestNewS3Uploader_SSRFRejected(t *testing.T) {
	_, err := newS3Uploader(S3Config{Endpoint: "169.254.169.254", Bucket: "b"}, S3Secrets{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SSRF validation")
}

func TestNewS3Uploader_EndpointWithPort_SSRFAppliesToHostOnly(t *testing.T) {
	withPermissiveSSRFForLocalTest(t)
	uploader, err := newS3Uploader(S3Config{Endpoint: "203.0.113.10:9000", Bucket: "b"}, S3Secrets{})
	require.NoError(t, err)
	assert.NotNil(t, uploader)
}

func TestNewS3Uploader_ForcePathStyle(t *testing.T) {
	withPermissiveSSRFForLocalTest(t)
	uploader, err := newS3Uploader(S3Config{Endpoint: "203.0.113.10", Bucket: "b", ForcePathStyle: true}, S3Secrets{})
	require.NoError(t, err)
	assert.NotNil(t, uploader)
}

// TestNewS3Uploader_ClientCreationFails proves minio.New's own error branch
// (s3.go's "s3: create client" wrap) is reachable and wrapped: an endpoint
// containing a raw control byte passes SSRF validation (which only
// resolves/inspects the host, done here via withPermissiveSSRFForLocalTest
// so the test targets client construction specifically) but fails
// minio-go's own internal URL parsing, which happens inside minio.New
// itself before any network I/O.
func TestNewS3Uploader_ClientCreationFails(t *testing.T) {
	withPermissiveSSRFForLocalTest(t)

	_, err := newS3Uploader(S3Config{Endpoint: "bad endpoint with a\x00null byte", Bucket: "b"}, S3Secrets{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "s3: create client")
}

// NOTE: Upload's f.Stat() error branch (s3.go ~104-105) is not covered here.
// Upload takes only a localPath string and always performs its own
// successful os.Open internally, so forcing the immediately-following
// f.Stat() to fail on that freshly (and successfully) opened descriptor
// would require either a TOCTOU race on the underlying inode (Linux does
// not invalidate fstat() on an open fd even after unlink, so deleting the
// file mid-flight does not reproduce this) or a production-code seam to
// inject a pre-broken file handle, which this test-only pass must not add.
