package immich

import (
	"context"

	"github.com/sweepies/immich-go/internal/filetypes"
)

// ServerService provides server-level operations.
type ServerService interface {
	// PingServer checks if the server is reachable.
	PingServer(ctx context.Context) error

	// ValidateConnection validates the API key and returns user info.
	ValidateConnection(ctx context.Context) (User, error)

	// GetServerStatistics returns server-wide statistics.
	GetServerStatistics(ctx context.Context) (ServerStatistics, error)

	// GetAboutInfo returns server version and build information.
	GetAboutInfo(ctx context.Context) (AboutInfo, error)

	// SupportedMedia returns the media types supported by the server.
	SupportedMedia() filetypes.SupportedMedia
}

// User represents an Immich user.
type User struct {
	ID               UserID `json:"id"`
	Email            string `json:"email"`
	Name             string `json:"name"`
	ProfileImagePath string `json:"profileImagePath"`
	IsAdmin          bool   `json:"isAdmin"`
}

// ServerStatistics contains server-wide statistics.
type ServerStatistics struct {
	Photos      int   `json:"photos"`
	Videos      int   `json:"videos"`
	Usage       int64 `json:"usage"`
	UsageByUser []struct {
		UserID           UserID `json:"userId"`
		UserName         string `json:"userName"`
		Photos           int    `json:"photos"`
		Videos           int    `json:"videos"`
		Usage            int64  `json:"usage"`
		QuotaSizeInBytes any    `json:"quotaSizeInBytes"`
	} `json:"usageByUser"`
}

// AboutInfo contains server version and build information.
type AboutInfo struct {
	Version       string `json:"version"`
	VersionURL    string `json:"versionUrl"`
	Licensed      bool   `json:"licensed"`
	Build         string `json:"build"`
	BuildURL      string `json:"buildUrl"`
	BuildImage    string `json:"buildImage"`
	BuildImageURL string `json:"buildImageUrl"`
	Repository    string `json:"repository"`
	RepositoryURL string `json:"repositoryUrl"`
	SourceRef     string `json:"sourceRef"`
	SourceCommit  string `json:"sourceCommit"`
	SourceURL     string `json:"sourceUrl"`
	Nodejs        string `json:"nodejs"`
	Exiftool      string `json:"exiftool"`
	Ffmpeg        string `json:"ffmpeg"`
	Libvips       string `json:"libvips"`
	Imagemagick   string `json:"imagemagick"`
}
