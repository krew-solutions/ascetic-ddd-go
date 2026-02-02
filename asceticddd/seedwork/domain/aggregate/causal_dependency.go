package aggregate

func NewCausalDependency(
	tenantId any,
	streamType string,
	streamId any,
	streamPosition uint,
) (CausalDependency, error) {
	/*
	   This is enough to extract aggregate with required version from repository.
	   And this is enough to check causal dependencies in Inbox.
	*/
	return CausalDependency{
		tenantId:       tenantId,
		streamType:     streamType,
		streamId:       streamId,
		streamPosition: streamPosition,
	}, nil
}

type CausalDependency struct {
	tenantId       any    // aggregate.id.tenant_id
	streamType     string // bounded_context_name.aggregate_name
	streamId       any    // aggregate.id.internal_id
	streamPosition uint   // aggregate.version
}

func (d CausalDependency) TenantId() any {
	return d.tenantId
}

func (d CausalDependency) StreamId() any {
	return d.streamId
}

func (d CausalDependency) StreamType() string {
	return d.streamType
}

func (d CausalDependency) StreamPosition() uint {
	return d.streamPosition
}

func (d CausalDependency) Export(ex CausalDependencyExporterSetter) {
	ex.SetStreamId(d.tenantId)
	ex.SetStreamType(d.streamType)
	ex.SetStreamId(d.streamId)
	ex.SetStreamPosition(d.streamPosition)
}

type CausalDependencyExporterSetter interface {
	SetTenantId(any)
	SetStreamType(string)
	SetStreamId(any)
	SetStreamPosition(uint)
}

type CausalDependencyExporter struct {
	TenantId       any
	StreamType     string
	StreamId       any
	StreamPosition uint
}

func (ex *CausalDependencyExporter) SetTenantId(val any) {
	ex.TenantId = val
}

func (ex *CausalDependencyExporter) SetStreamId(val any) {
	ex.StreamId = val
}
func (ex *CausalDependencyExporter) SetStreamType(val string) {
	ex.StreamType = val
}
func (ex *CausalDependencyExporter) SetStreamPosition(val uint) {
	ex.StreamPosition = val
}

type CausalDependencyReconstitutor struct {
	TenantId       any
	StreamType     string
	StreamId       any
	StreamPosition uint
}

func (r CausalDependencyReconstitutor) Reconstitute() (CausalDependency, error) {
	return NewCausalDependency(r.TenantId, r.StreamType, r.StreamId, r.StreamPosition)
}
