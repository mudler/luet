package imgworker

// FROM Slightly adapted from genuinetools/img worker

import (
	"context"

	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/filesync"
	"github.com/moby/buildkit/session/testutil"
	"github.com/pkg/errors"
)

func (c *Client) getSessionManager() (*session.Manager, error) {
	if c.sessionManager == nil {
		var err error
		c.sessionManager, err = session.NewManager()
		if err != nil {
			return nil, err
		}
	}
	return c.sessionManager, nil
}

// Session creates the session manager and returns the session and it's
// dialer.
func (c *Client) Session(ctx context.Context) (*session.Session, session.Dialer, error) {
	m, err := c.getSessionManager()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create session manager")
	}
	sessionName := "luet"
	s, err := session.NewSession(ctx, sessionName, "")
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create session")
	}
	syncedDirs := make([]filesync.SyncedDir, 0, len(c.localDirs))
	for name, d := range c.localDirs {
		syncedDirs = append(syncedDirs, filesync.SyncedDir{Name: name, Dir: d})
	}
	s.Allow(filesync.NewFSSyncProvider(syncedDirs))
	s.Allow(NewDockerAuthProvider(c.auth))
	return s, sessionDialer(s, m), err
}

func sessionDialer(s *session.Session, m *session.Manager) session.Dialer {
	// FIXME: rename testutil
	return session.Dialer(testutil.TestStream(testutil.Handler(m.HandleConn)))
}
