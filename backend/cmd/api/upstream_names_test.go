package main

import (
	"encoding/binary"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/services"
)

// recordingUpstreamManager records the resolver it is given.
type recordingUpstreamManager struct {
	resolvers []caddy.UpstreamResolver
}

func (m *recordingUpstreamManager) SetUpstreamResolver(r caddy.UpstreamResolver) {
	m.resolvers = append(m.resolvers, r)
}

func TestStartUpstreamNames_BindFailureLeavesTheManagerWithoutAResolver(t *testing.T) {
	hook := logtest.NewLocal(logger.Log().Logger)
	t.Cleanup(hook.Reset)
	mgr := &recordingUpstreamManager{}

	server := startUpstreamNames(mgr, func() (*services.UpstreamNameServer, error) {
		return nil, errors.New("bind refused")
	})

	assert.Nil(t, server)
	assert.Empty(t, mgr.resolvers, "no resolver is set when the answerer does not start")
	require.NotEmpty(t, hook.AllEntries())
	last := hook.LastEntry()
	assert.Equal(t, "Upstream name checks are not available; forward target names are checked only when saved", last.Message)
	assert.Equal(t, "error", last.Level.String())
}

func TestStartUpstreamNames_SetsTheResolverAndSurvivesCancelledContext(t *testing.T) {
	mgr := &recordingUpstreamManager{}
	server := startUpstreamNames(mgr, services.StartUpstreamNameServer)
	require.NotNil(t, server)
	defer func() { _ = server.Close() }()
	require.Len(t, mgr.resolvers, 1)
	assert.True(t, strings.HasPrefix(mgr.resolvers[0].General, "udp/127.0.0.1:"))
	assert.NotEmpty(t, mgr.resolvers[0].Reserved)

	// The answerer takes no context, so nothing a shutdown cancels stops it: it still replies until Close.
	assert.NotNil(t, udpQuery(t, mgr.resolvers[0].General, "localhost."), "the answerer replies after the context is cancelled")
}

// udpQuery sends an A query to the answerer and returns the reply, or nil on timeout.
func udpQuery(t *testing.T, address, name string) []byte {
	t.Helper()
	nameWire, err := dnsmessage.NewName(name)
	require.NoError(t, err)
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{ID: 0x4242, RecursionDesired: true})
	require.NoError(t, b.StartQuestions())
	require.NoError(t, b.Question(dnsmessage.Question{Name: nameWire, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}))
	packet, err := b.Finish()
	require.NoError(t, err)

	conn, err := net.Dial("udp", strings.TrimPrefix(address, "udp/"))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	require.NoError(t, conn.SetDeadline(time.Now().Add(6*time.Second)))
	_, err = conn.Write(packet)
	require.NoError(t, err)
	buf := make([]byte, 1500)
	n, err := conn.Read(buf)
	if err != nil {
		return nil
	}
	require.GreaterOrEqual(t, n, 2)
	assert.Equal(t, uint16(0x4242), binary.BigEndian.Uint16(buf[:2]))
	return buf[:n]
}
