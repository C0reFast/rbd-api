package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"ichenfu.com/rbd-api/pkg/types"

	"github.com/ceph/go-ceph/rados"
	"github.com/ceph/go-ceph/rbd"
	"github.com/emicklei/go-restful"
)

const SIZE_GB = 1024 * 1024 * 1024
const RBD_META_BPS_LIMIT = "conf_rbd_qos_bps_limit"
const RBD_META_IOPS_LIMIT = "conf_rbd_qos_iops_limit"

type RbdHander struct {
	client *rados.Conn
}

func NewRbdHander(client *rados.Conn) *RbdHander {
	h := &RbdHander{client: client}
	return h
}

func (h *RbdHander) WebService() *restful.WebService {
	ws := new(restful.WebService)
	ws.Path("/v1/rbd").Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)
	ws.Route(ws.POST("/pools/{pool}/images").To(h.createImage).Reads(types.Image{}))
	ws.Route(ws.GET("/pools/{pool}/images/{imageName}").To(h.getImage))
	ws.Route(ws.PUT("/pools/{pool}/images/{imageName}").To(h.updateImage))
	ws.Route(ws.DELETE("/pools/{pool}/images/{imageName}").To(h.deleteImage))
	ws.Route(ws.GET("/pools/{pool}/images/").To(h.listImage))

	ws.Route(ws.POST("/pools/{pool}/images/{imageName}/snapshots").To(h.createSnapshot).Reads(types.Snapshot{}))
	ws.Route(ws.GET("/pools/{pool}/images/{imageName}/snapshots/{snapshotName}").To(h.getSnapshot))
	ws.Route(ws.GET("/pools/{pool}/images/{imageName}/snapshots/").To(h.listSnapshot))
	ws.Route(ws.POST("/pools/{pool}/images/{imageName}/snapshots/{snapshotName}/clone").To(h.createClone).Reads(types.Image{}))

	return ws
}

func (h *RbdHander) createImage(request *restful.Request, response *restful.Response) {
	image := types.Image{Pool: request.PathParameter("pool")}
	err := request.ReadEntity(&image)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	if image.Name == "" || image.Size == 0 {
		response.WriteErrorString(http.StatusBadRequest, "name, size is required")
		return
	}
	ctx, err := h.client.OpenIOContext(image.Pool)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	defer ctx.Destroy()
	if err := rbd.CreateImage(ctx, image.Name, image.Size*SIZE_GB, rbd.NewRbdImageOptions()); err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	response.WriteHeaderAndEntity(http.StatusCreated, image)
}

func (h *RbdHander) getImage(request *restful.Request, response *restful.Response) {
	image := types.Image{Pool: request.PathParameter("pool"), Name: request.PathParameter("imageName")}
	ctx, err := h.client.OpenIOContext(image.Pool)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	defer ctx.Destroy()
	rbdImage, err := rbd.OpenImageReadOnly(ctx, image.Name, rbd.NoSnapshot)
	if err != nil {
		if err == rbd.ErrNotFound {
			response.WriteError(http.StatusNotFound, fmt.Errorf("image %s is not found", image.Name))
			return
		}
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	image.Size, err = rbdImage.GetSize()
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	image.Size /= SIZE_GB
	qosBPS, _ := rbdImage.GetMetadata(RBD_META_BPS_LIMIT)
	qosIOPS, _ := rbdImage.GetMetadata(RBD_META_IOPS_LIMIT)
	image.QosBPS, _ = strconv.ParseInt(qosBPS, 10, 64)
	image.QosIOPS, _ = strconv.ParseInt(qosIOPS, 10, 64)

	response.WriteAsJson(image)
}

func (h *RbdHander) deleteImage(request *restful.Request, response *restful.Response) {
	image := types.Image{Pool: request.PathParameter("pool"), Name: request.PathParameter("imageName")}
	ctx, err := h.client.OpenIOContext(image.Pool)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	defer ctx.Destroy()
	if err := rbd.RemoveImage(ctx, image.Name); err != nil {
		if err == rbd.ErrNotFound {
			response.WriteError(http.StatusNotFound, fmt.Errorf("image %s is not found", image.Name))
			return
		}
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	response.WriteAsJson(struct{}{})
}

func (h *RbdHander) updateImage(request *restful.Request, response *restful.Response) {
	image := types.Image{Pool: request.PathParameter("pool"), Name: request.PathParameter("imageName")}
	err := request.ReadEntity(&image)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	ctx, err := h.client.OpenIOContext(image.Pool)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	defer ctx.Destroy()
	rbdImage, err := rbd.OpenImage(ctx, image.Name, rbd.NoSnapshot)
	if err != nil {
		if err == rbd.ErrNotFound {
			response.WriteError(http.StatusNotFound, fmt.Errorf("image %s is not found", image.Name))
			return
		}
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	if image.Size != 0 {
		if err := rbdImage.Resize(image.Size * SIZE_GB); err != nil {
			response.WriteError(http.StatusInternalServerError, err)
			return
		}
	}
	if image.QosBPS != 0 {
		if err := rbdImage.SetMetadata(RBD_META_BPS_LIMIT, fmt.Sprintf("%d", image.QosBPS)); err != nil {
			response.WriteError(http.StatusInternalServerError, err)
			return
		}
	}
	if image.QosIOPS != 0 {
		if err := rbdImage.SetMetadata(RBD_META_IOPS_LIMIT, fmt.Sprintf("%d", image.QosIOPS)); err != nil {
			response.WriteError(http.StatusInternalServerError, err)
			return
		}
	}

	response.WriteEntity(image)
}

func (h *RbdHander) listImage(request *restful.Request, response *restful.Response) {
	image := types.Image{Pool: request.PathParameter("pool")}
	ctx, err := h.client.OpenIOContext(image.Pool)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	defer ctx.Destroy()
	images, err := rbd.GetImageNames(ctx)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	list := []types.Image{}
	for _, img := range images {
		list = append(list, types.Image{Name: img, Pool: image.Pool})
	}

	response.WriteEntity(list)
}

func (h *RbdHander) createSnapshot(request *restful.Request, response *restful.Response) {
	snap := types.Snapshot{Pool: request.PathParameter("pool"), ImageName: request.PathParameter("imageName")}

	err := request.ReadEntity(&snap)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	if snap.Name == "" {
		response.WriteErrorString(http.StatusBadRequest, "name is required")
		return
	}
	ctx, err := h.client.OpenIOContext(snap.Pool)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	defer ctx.Destroy()
	rbdImage, err := rbd.OpenImage(ctx, snap.Name, rbd.NoSnapshot)
	if err != nil {
		if err == rbd.ErrNotFound {
			response.WriteError(http.StatusNotFound, fmt.Errorf("image %s is not found", snap.ImageName))
			return
		}
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	snapshot, err := rbdImage.CreateSnapshot(snap.Name)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	if err := snapshot.Protect(); err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	response.WriteEntity(snap)
}

func (h *RbdHander) getSnapshot(request *restful.Request, response *restful.Response) {
	snap := types.Snapshot{Pool: request.PathParameter("pool"), ImageName: request.PathParameter("imageName"), Name: request.PathParameter("snapshotName")}
	ctx, err := h.client.OpenIOContext(snap.Pool)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	defer ctx.Destroy()
	_, err = rbd.OpenImageReadOnly(ctx, snap.ImageName, snap.Name)
	if err != nil {
		if err == rbd.ErrNotFound {
			response.WriteError(http.StatusNotFound, fmt.Errorf("snapshot %s@%s is not found", snap.ImageName, snap.Name))
			return
		}
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	response.WriteEntity(snap)
}

func (h *RbdHander) listSnapshot(request *restful.Request, response *restful.Response) {
	snap := types.Snapshot{Pool: request.PathParameter("pool"), ImageName: request.PathParameter("imageName")}
	ctx, err := h.client.OpenIOContext(snap.Pool)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	defer ctx.Destroy()
	rbdImage, err := rbd.OpenImageReadOnly(ctx, snap.ImageName, rbd.NoSnapshot)
	if err != nil {
		if err == rbd.ErrNotFound {
			response.WriteError(http.StatusNotFound, fmt.Errorf("image %s is not found", snap.ImageName))
			return
		}
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	snaps, err := rbdImage.GetSnapshotNames()
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	list := []types.Snapshot{}
	for _, s := range snaps {
		list = append(list, types.Snapshot{Pool: snap.Pool, ImageName: snap.ImageName, Name: s.Name})
	}

	response.WriteEntity(list)
}

func (h *RbdHander) createClone(request *restful.Request, response *restful.Response) {
	snap := types.Snapshot{Pool: request.PathParameter("pool"), ImageName: request.PathParameter("imageName"), Name: request.PathParameter("snapshotName")}
	image := types.Image{}
	err := request.ReadEntity(&image)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	if image.Name == "" {
		response.WriteErrorString(http.StatusBadRequest, "name is required")
		return
	}
	ctx, err := h.client.OpenIOContext(snap.Pool)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	defer ctx.Destroy()
	snapshot, err := rbd.OpenImageReadOnly(ctx, snap.ImageName, snap.Name)
	if err != nil {
		if err == rbd.ErrNotFound {
			response.WriteError(http.StatusNotFound, fmt.Errorf("snapshot %s@%s is not found", snap.ImageName, snap.Name))
			return
		}
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	if err := rbd.CloneFromImage(snapshot, snap.Name, ctx, image.Name, rbd.NewRbdImageOptions()); err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	image.Pool = snap.Pool
	image.Size, _ = snapshot.GetSize()
	image.Size /= SIZE_GB
	response.WriteEntity(image)
}
