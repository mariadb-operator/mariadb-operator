package backup

import "testing"

func TestGetFilePath(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		fileName     string
		wantFilePath string
	}{
		{
			name:         "empty path",
			path:         "",
			fileName:     "backup.2023-12-22T13:00:00Z.foo.sql",
			wantFilePath: "backup.2023-12-22T13:00:00Z.foo.sql",
		},
		{
			name:         "add path",
			path:         "/backup",
			fileName:     "backup.2023-12-22T13:00:00Z.foo.sql",
			wantFilePath: "/backup/backup.2023-12-22T13:00:00Z.foo.sql",
		},
		{
			name:         "already has relative path",
			path:         "/backup",
			fileName:     "mariadb/backup.2023-12-22T13:00:00Z.foo.sql",
			wantFilePath: "/backup/mariadb/backup.2023-12-22T13:00:00Z.foo.sql",
		},
		{
			name:         "already has absolute path",
			path:         "/backup",
			fileName:     "/backup/backup.2023-12-22T13:00:00Z.foo.sql",
			wantFilePath: "/backup/backup.2023-12-22T13:00:00Z.foo.sql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := GetFilePath(tt.path, tt.fileName)
			if tt.wantFilePath != filePath {
				t.Fatalf("unexpected file path, expected: %v got: %v", tt.wantFilePath, filePath)
			}
		})
	}
}
