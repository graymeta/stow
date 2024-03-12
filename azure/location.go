package azure

import (
	"context"
	"errors"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	azcontainer "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"net/url"
	"strings"
	"time"

	"github.com/flyteorg/stow"
)

type location struct {
	accountName       string
	uploadConcurrency int
	client            *azblob.Client
	preSigner         RequestPreSigner
}

func (l *location) Close() error {
	return nil // nothing to close
}

var publicAccessTypeContainer = azcontainer.PublicAccessTypeContainer

// CreateContainer follows the contract from stow.Location, with one notable opinion.
// Attempts to create an already-existing container will not produce an error.
func (l *location) CreateContainer(name string) (stow.Container, error) {
	ctx := context.Background()
	resp, err := l.client.CreateContainer(
		ctx,
		name,
		&azblob.CreateContainerOptions{Access: &publicAccessTypeContainer})

	if err != nil {
		var tErr *azcore.ResponseError
		ok := errors.As(err, &tErr)
		// Note: StatusConflict (409) is used for both "already exists"
		// and "deleting" failures.
		if ok &&
			tErr.StatusCode == http.StatusConflict &&
			tErr.ErrorCode == "ContainerAlreadyExists" {
			return l.Container(name)
		}
		return nil, err
	}

	container := &container{
		id: name,
		properties: &BlobProps{
			ETag:         *resp.ETag,
			LastModified: *resp.LastModified,
		},
		client:            l.client.ServiceClient().NewContainerClient(name),
		preSigner:         l.preSigner,
		uploadConcurrency: l.uploadConcurrency,
	}
	// TK: What is this here for? Presumably to wait for the container to
	// really be available. If that's the case, a validation mechanism is
	// a much better path if you want this to always work.
	time.Sleep(time.Second * 3)
	return container, nil
}

func (l *location) Containers(prefix, cursor string, count int) ([]stow.Container, string, error) {
	ctx := context.Background()
	params := azblob.ListContainersOptions{
		MaxResults: to.Ptr(int32(count)),
		Prefix:     &prefix,
	}
	if cursor != stow.CursorStart {
		params.Marker = &cursor
	}

	pager := l.client.NewListContainersPager(&params)
	resp, err := pager.NextPage(ctx)
	if err != nil {
		return nil, cursor, err
	}

	stowContainers := make([]stow.Container, len(resp.ContainerItems))
	for i, azContainer := range resp.ContainerItems {
		stowContainers[i] = &container{
			id: *azContainer.Name,
			properties: &BlobProps{
				ETag:         *azContainer.Properties.ETag,
				LastModified: *azContainer.Properties.LastModified,
			},
			client:            l.client.ServiceClient().NewContainerClient(*azContainer.Name),
			preSigner:         l.preSigner,
			uploadConcurrency: l.uploadConcurrency,
		}
	}

	return stowContainers, *resp.NextMarker, nil
}

func (l *location) Container(id string) (stow.Container, error) {
	cursor := stow.CursorStart
	for {
		containers, crsr, err := l.Containers(id[:3], cursor, 100)
		if err != nil {
			return nil, stow.ErrNotFound
		}
		for _, i := range containers {
			if i.ID() == id {
				return i, nil
			}
		}

		cursor = crsr
		if cursor == "" {
			break
		}
	}

	return nil, stow.ErrNotFound
}

func (l *location) ItemByURL(url *url.URL) (stow.Item, error) {
	if url.Scheme != "azure" {
		return nil, errors.New("not valid azure URL")
	}

	locationAccountPart := strings.Split(url.Host, ".")[0]
	if locationAccountPart != l.accountName {
		return nil, errors.New("wrong azure URL")
	}

	path := strings.TrimLeft(url.Path, "/")
	params := strings.SplitN(path, "/", 2)
	if len(params) != 2 {
		return nil, errors.New("wrong path")
	}
	c, err := l.Container(params[0])
	if err != nil {
		return nil, err
	}
	return c.Item(params[1])
}

func (l *location) RemoveContainer(id string) error {
	ctx := context.Background()
	_, err := l.client.DeleteContainer(ctx, id, &azblob.DeleteContainerOptions{})
	return err
}
