package types

type Image struct {
	Pool    string `json:"pool" description:"Pool Name"`
	Name    string `json:"name" description:"Image Name"`
	Size    uint64 `json:"size" description:"Image Size in GB"`
	QosBPS  int64  `json:"qos_bps" description:"Bandwidh Limit"`
	QosIOPS int64  `json:"qos_iops" description:"IOPS Limit"`
}

type Snapshot struct {
	Pool      string `json:"pool" description:"Pool Name"`
	ImageName string `json:"image_name" description:"Image Name"`
	Name      string `json:"name" description:"Snapshot Name"`
}
