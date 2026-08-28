package immich

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/sweepies/immich-go/internal/assets"
)

type AlbumSimplified struct {
	ID          AlbumID `json:"id,omitempty"`
	AlbumName   string  `json:"albumName"`
	Description string  `json:"description,omitempty"`
}

func AlbumsFromAlbumSimplified(albums []AlbumSimplified) []assets.Album {
	result := make([]assets.Album, 0, len(albums))
	for _, a := range albums {
		result = append(result, assets.Album{
			ID:          a.ID.String(),
			Title:       a.AlbumName,
			Description: a.Description,
		})
	}
	return result
}

func (ic *ImmichClient) GetAllAlbums(ctx context.Context) ([]AlbumSimplified, error) {
	var albums []AlbumSimplified
	err := ic.newServerCall(ctx, EndPointGetAllAlbums).
		do(
			getRequest("/albums", setAcceptJSON()),
			responseJSON(&albums),
		)
	if err != nil {
		return nil, err
	}
	return albums, nil
}

type AlbumContent struct {
	ID          AlbumID `json:"id,omitempty"`
	AlbumName   string  `json:"albumName"`
	Description string  `json:"description"`
	Shared      bool    `json:"shared"`
}

func (ic *ImmichClient) GetAlbumInfo(ctx context.Context, id AlbumID, withoutAssets bool) (AlbumContent, error) {
	var album AlbumContent
	query := id.String()
	if withoutAssets {
		query += "?withoutAssets=true"
	} else {
		query += "?withoutAssets=false"
	}
	err := ic.newServerCall(ctx, EndPointGetAlbumInfo).do(getRequest("/albums/"+query, setAcceptJSON()), responseJSON(&album))
	return album, err
}

func (ic *ImmichClient) GetAssetsAlbums(ctx context.Context, id AssetID) ([]assets.Album, error) {
	var albums []AlbumSimplified
	err := ic.newServerCall(ctx, EndPointGetAlbumInfo).do(getRequest("/albums", setAcceptJSON()), responseJSON(&albums))
	if err != nil {
		return nil, err
	}
	return AlbumsFromAlbumSimplified(albums), nil
}

type UpdateAlbum struct {
	IDs []AssetID `json:"ids"`
}

type UpdateAlbumResult struct {
	ID      AssetID `json:"id"`
	Success bool    `json:"success"`
	Error   string  `json:"error,omitempty"`
}

func (ic *ImmichClient) AddAssetToAlbum(ctx context.Context, albumID AlbumID, assetIDs []AssetID) ([]UpdateAlbumResult, error) {
	if ic.dryRun {
		return []UpdateAlbumResult{}, nil
	}
	var result []UpdateAlbumResult
	body := UpdateAlbum{
		IDs: assetIDs,
	}
	err := ic.newServerCall(ctx, EndPointAddAsstToAlbum).do(
		putRequest(fmt.Sprintf("/albums/%s/assets", albumID), setAcceptJSON(),
			setJSONBody(body)),
		responseJSON(&result))
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (ic *ImmichClient) CreateAlbum(ctx context.Context, name, description string, assetIDs []AssetID) (assets.Album, error) {
	if ic.dryRun {
		return assets.Album{
			ID:    uuid.NewString(),
			Title: name,
		}, nil
	}
	body := struct {
		AlbumName   string    `json:"albumName"`
		Description string    `json:"description"`
		AssetIDs    []AssetID `json:"assetIds,omitempty"`
	}{
		AlbumName:   name,
		Description: description,
		AssetIDs:    assetIDs,
	}
	var result AlbumSimplified
	err := ic.newServerCall(ctx, EndPointCreateAlbum).do(
		postRequest("/albums", "application/json", setAcceptJSON(), setJSONBody(body)),
		responseJSON(&result))
	if err != nil {
		return assets.Album{}, err
	}
	return assets.Album{
		ID:          result.ID.String(),
		Title:       result.AlbumName,
		Description: result.Description,
	}, nil
}

func (ic *ImmichClient) GetAssetAlbums(ctx context.Context, assetID AssetID) ([]AlbumSimplified, error) {
	var result []AlbumSimplified
	err := ic.newServerCall(ctx, EndPointGetAssetAlbums).do(
		getRequest("/albums?assetId="+assetID.String(), setAcceptJSON()),
		responseJSON(&result))
	return result, err
}

func (ic *ImmichClient) DeleteAlbum(ctx context.Context, id AlbumID) error {
	if ic.dryRun {
		return nil
	}
	return ic.newServerCall(ctx, EndPointDeleteAlbum).do(deleteRequest("/albums/" + id.String()))
}
