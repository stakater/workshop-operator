//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BookbagSpec) DeepCopyInto(out *BookbagSpec) {
	*out = *in
	out.Image = in.Image
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BookbagSpec.
func (in *BookbagSpec) DeepCopy() *BookbagSpec {
	if in == nil {
		return nil
	}
	out := new(BookbagSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CertManagerSpec) DeepCopyInto(out *CertManagerSpec) {
	*out = *in
	out.OperatorHub = in.OperatorHub
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CertManagerSpec.
func (in *CertManagerSpec) DeepCopy() *CertManagerSpec {
	if in == nil {
		return nil
	}
	out := new(CertManagerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CodeReadyWorkspaceSpec) DeepCopyInto(out *CodeReadyWorkspaceSpec) {
	*out = *in
	out.OperatorHub = in.OperatorHub
	out.PluginRegistryImage = in.PluginRegistryImage
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CodeReadyWorkspaceSpec.
func (in *CodeReadyWorkspaceSpec) DeepCopy() *CodeReadyWorkspaceSpec {
	if in == nil {
		return nil
	}
	out := new(CodeReadyWorkspaceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GitOpsSpec) DeepCopyInto(out *GitOpsSpec) {
	*out = *in
	out.OperatorHub = in.OperatorHub
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GitOpsSpec.
func (in *GitOpsSpec) DeepCopy() *GitOpsSpec {
	if in == nil {
		return nil
	}
	out := new(GitOpsSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GiteaSpec) DeepCopyInto(out *GiteaSpec) {
	*out = *in
	out.Image = in.Image
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GiteaSpec.
func (in *GiteaSpec) DeepCopy() *GiteaSpec {
	if in == nil {
		return nil
	}
	out := new(GiteaSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GuideSpec) DeepCopyInto(out *GuideSpec) {
	*out = *in
	out.Bookbag = in.Bookbag
	in.Scholars.DeepCopyInto(&out.Scholars)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GuideSpec.
func (in *GuideSpec) DeepCopy() *GuideSpec {
	if in == nil {
		return nil
	}
	out := new(GuideSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ImageSpec) DeepCopyInto(out *ImageSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImageSpec.
func (in *ImageSpec) DeepCopy() *ImageSpec {
	if in == nil {
		return nil
	}
	out := new(ImageSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InfrastructureSpec) DeepCopyInto(out *InfrastructureSpec) {
	*out = *in
	out.CertManager = in.CertManager
	out.CodeReadyWorkspace = in.CodeReadyWorkspace
	out.Gitea = in.Gitea
	out.GitOps = in.GitOps
	in.Guide.DeepCopyInto(&out.Guide)
	out.Nexus = in.Nexus
	out.Pipeline = in.Pipeline
	out.Project = in.Project
	out.ServiceMesh = in.ServiceMesh
	out.Serverless = in.Serverless
	out.Vault = in.Vault
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InfrastructureSpec.
func (in *InfrastructureSpec) DeepCopy() *InfrastructureSpec {
	if in == nil {
		return nil
	}
	out := new(InfrastructureSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NexusSpec) DeepCopyInto(out *NexusSpec) {
	*out = *in
	out.Image = in.Image
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NexusSpec.
func (in *NexusSpec) DeepCopy() *NexusSpec {
	if in == nil {
		return nil
	}
	out := new(NexusSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OperatorHubSpec) DeepCopyInto(out *OperatorHubSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OperatorHubSpec.
func (in *OperatorHubSpec) DeepCopy() *OperatorHubSpec {
	if in == nil {
		return nil
	}
	out := new(OperatorHubSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineSpec) DeepCopyInto(out *PipelineSpec) {
	*out = *in
	out.OperatorHub = in.OperatorHub
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineSpec.
func (in *PipelineSpec) DeepCopy() *PipelineSpec {
	if in == nil {
		return nil
	}
	out := new(PipelineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectSpec) DeepCopyInto(out *ProjectSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectSpec.
func (in *ProjectSpec) DeepCopy() *ProjectSpec {
	if in == nil {
		return nil
	}
	out := new(ProjectSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScholarsSpec) DeepCopyInto(out *ScholarsSpec) {
	*out = *in
	if in.GuideURL != nil {
		in, out := &in.GuideURL, &out.GuideURL
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScholarsSpec.
func (in *ScholarsSpec) DeepCopy() *ScholarsSpec {
	if in == nil {
		return nil
	}
	out := new(ScholarsSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServerlessSpec) DeepCopyInto(out *ServerlessSpec) {
	*out = *in
	out.OperatorHub = in.OperatorHub
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServerlessSpec.
func (in *ServerlessSpec) DeepCopy() *ServerlessSpec {
	if in == nil {
		return nil
	}
	out := new(ServerlessSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceMeshSpec) DeepCopyInto(out *ServiceMeshSpec) {
	*out = *in
	out.ServiceMeshOperatorHub = in.ServiceMeshOperatorHub
	out.ElasticSearchOperatorHub = in.ElasticSearchOperatorHub
	out.JaegerOperatorHub = in.JaegerOperatorHub
	out.KialiOperatorHub = in.KialiOperatorHub
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceMeshSpec.
func (in *ServiceMeshSpec) DeepCopy() *ServiceMeshSpec {
	if in == nil {
		return nil
	}
	out := new(ServiceMeshSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SourceSpec) DeepCopyInto(out *SourceSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SourceSpec.
func (in *SourceSpec) DeepCopy() *SourceSpec {
	if in == nil {
		return nil
	}
	out := new(SourceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UserDetailsSpec) DeepCopyInto(out *UserDetailsSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UserDetailsSpec.
func (in *UserDetailsSpec) DeepCopy() *UserDetailsSpec {
	if in == nil {
		return nil
	}
	out := new(UserDetailsSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VaultSpec) DeepCopyInto(out *VaultSpec) {
	*out = *in
	out.Image = in.Image
	out.AgentInjectorImage = in.AgentInjectorImage
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VaultSpec.
func (in *VaultSpec) DeepCopy() *VaultSpec {
	if in == nil {
		return nil
	}
	out := new(VaultSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Workshop) DeepCopyInto(out *Workshop) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Workshop.
func (in *Workshop) DeepCopy() *Workshop {
	if in == nil {
		return nil
	}
	out := new(Workshop)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Workshop) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkshopList) DeepCopyInto(out *WorkshopList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Workshop, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkshopList.
func (in *WorkshopList) DeepCopy() *WorkshopList {
	if in == nil {
		return nil
	}
	out := new(WorkshopList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WorkshopList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkshopSpec) DeepCopyInto(out *WorkshopSpec) {
	*out = *in
	out.Source = in.Source
	in.Infrastructure.DeepCopyInto(&out.Infrastructure)
	out.UserDetails = in.UserDetails
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkshopSpec.
func (in *WorkshopSpec) DeepCopy() *WorkshopSpec {
	if in == nil {
		return nil
	}
	out := new(WorkshopSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkshopStatus) DeepCopyInto(out *WorkshopStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkshopStatus.
func (in *WorkshopStatus) DeepCopy() *WorkshopStatus {
	if in == nil {
		return nil
	}
	out := new(WorkshopStatus)
	in.DeepCopyInto(out)
	return out
}
