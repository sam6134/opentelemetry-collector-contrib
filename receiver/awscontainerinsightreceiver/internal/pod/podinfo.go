package pod

type Info struct {
	namespace string
	podName   string
}

// GetNamespace returns the namespace for the pod
func (m *Info) GetNamespace() string {
	return m.namespace
}

// GetPodName returns the podName for the pod
func (m *Info) GetPodName() string {
	return m.podName
}

// NewInfo creates a new Info struct
func NewInfo() (*Info, error) {
	mInfo := &Info{
		podName:   "DummyPodName",
		namespace: "DummyNamespace",
	}

	return mInfo, nil
}
