package mysql

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDSNFromURL(t *testing.T) {
	got, err := dsnFromURL("mysql://ro:secret@db:3306/shop?tls=true")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(got, "ro:secret@tcp(db:3306)/shop"), "got %q", got)
	require.Contains(t, got, "tls=true")
	require.Contains(t, got, "parseTime=true")
}

func TestDSNFromURLDefaultsPort(t *testing.T) {
	got, err := dsnFromURL("mysql://u:p@host/db")
	require.NoError(t, err)
	require.Contains(t, got, "tcp(host:3306)")
}

func TestDSNFromURLPassesUnknownParams(t *testing.T) {
	got, err := dsnFromURL("mysql://u:p@host:3306/db?charset=utf8mb4")
	require.NoError(t, err)
	require.Contains(t, got, "charset=utf8mb4")
}
