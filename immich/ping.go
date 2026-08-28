package immich

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/sweepies/immich-go/internal/filetypes"
)

type PingResponse struct {
	Res string `json:"res"`
}

// Ping server
func (ic *ImmichClient) PingServer(ctx context.Context) error {
	r := PingResponse{}
	b := bytes.NewBuffer(nil)
	err := ic.newServerCall(ctx, EndPointPingServer).do(getRequest("/server/ping", setAcceptJSON()), responseCopy(b), responseJSON(&r))
	if err != nil {
		return fmt.Errorf("error while calling the immich's ping API at this address: %s:\n%s", ic.endPoint+"/server/ping", err.Error())
	}
	if r.Res != "pong" {
		return fmt.Errorf("unexpected response to the immich's ping API at this address: %s:\n%s", ic.endPoint+"/server/ping", b.String())
	}
	return nil
}

// ValidateConnection
// Validate the connection by querying the identity of the user having the given key

func (ic *ImmichClient) ValidateConnection(ctx context.Context) (User, error) {
	var user User

	err := ic.newServerCall(ctx, EndPointValidateConnection).
		do(getRequest("/users/me", setAcceptJSON()), responseJSON(&user))
	if err != nil {
		return user, err
	}

	sm, err := ic.GetSupportedMediaTypes(ctx)
	if err != nil {
		return user, err
	}
	ic.userID = user.ID
	ic.supportedMediaTypes = sm
	return user, nil
}

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

// getServerStatistics
// Get server stats

func (ic *ImmichClient) GetServerStatistics(ctx context.Context) (ServerStatistics, error) {
	var s ServerStatistics

	err := ic.newServerCall(ctx, EndPointGetServerStatistics).do(getRequest("/server/statistics", setAcceptJSON()), responseJSON(&s))
	return s, err
}

// ServerVersion is the parsed server version retained after GetAboutInfo.
// Only the major component is exposed because request compatibility is
// selected at the API generation boundary.
type ServerVersion struct {
	value string
	major uint64
}

// String returns the version exactly as reported by the server.
func (v ServerVersion) String() string {
	return v.value
}

// Major returns the server's API compatibility generation.
func (v ServerVersion) Major() uint64 {
	return v.major
}

func parseServerVersion(value string) (ServerVersion, error) {
	value = strings.TrimSpace(value)
	core := strings.TrimPrefix(value, "v")
	if i := strings.IndexAny(core, "+-"); i >= 0 {
		core = core[:i]
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return ServerVersion{}, fmt.Errorf("expected major.minor.patch")
	}
	components := make([]uint64, len(parts))
	for i, part := range parts {
		component, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return ServerVersion{}, fmt.Errorf("invalid component %q: %w", part, err)
		}
		components[i] = component
	}
	return ServerVersion{value: value, major: components[0]}, nil
}

// getAboutInfo
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

func (ic *ImmichClient) GetAboutInfo(ctx context.Context) (AboutInfo, error) {
	var about AboutInfo
	if err := ic.newServerCall(ctx, EndPointGetAboutInfo).do(getRequest("/server/about", setAcceptJSON()), responseJSON(&about)); err != nil {
		return about, err
	}
	version, err := parseServerVersion(about.Version)
	if err != nil {
		return about, fmt.Errorf("can't parse server version %q: %w", about.Version, err)
	}
	ic.serverVersion = version
	return about, nil
}

// getAssetStatistics
// Get user's stats

type AssetStatistics struct {
	Images int `json:"images"`
	Videos int `json:"videos"`
	Total  int `json:"total"`
}

func (ic *ImmichClient) GetAssetStatistics(ctx context.Context) (AssetStatistics, error) {
	var stats AssetStatistics
	err := ic.newServerCall(ctx, EndPointGetAssetStatistics).do(getRequest("/assets/statistics", setAcceptJSON()), responseJSON(&stats))
	return stats, err
}

func (ic *ImmichClient) GetSupportedMediaTypes(ctx context.Context) (filetypes.SupportedMedia, error) {
	var s map[string][]string

	err := ic.newServerCall(ctx, EndPointGetSupportedMediaTypes).do(getRequest("/server/media-types", setAcceptJSON()), responseJSON(&s))
	if err != nil {
		return nil, err
	}
	sm := make(filetypes.SupportedMedia)
	for t, l := range s {
		for _, e := range l {
			sm[e] = t
		}
	}
	sm[".mp"] = filetypes.TypeUseless
	sm[".json"] = filetypes.TypeSidecar
	return sm, err
}
