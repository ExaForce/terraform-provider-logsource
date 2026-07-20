package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/ExaForce/terraform-provider-logsource/internal/provider/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var _ resource.Resource = &AWSLogSourceEKSResource{}
var _ resource.ResourceWithImportState = &AWSLogSourceEKSResource{}

type AWSLogSourceEKSResource struct {
	client *client.Client
}

func NewAWSLogSourceEKSResource() resource.Resource {
	return &AWSLogSourceEKSResource{}
}

var eksSpecAttrTypes = map[string]attr.Type{
	"cluster_name": types.StringType,
	"eks_arn":      types.StringType,
}

type AWSLogSourceEKSResourceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	Spec types.Object `tfsdk:"spec"`
}

type EKSSpecModel struct {
	ClusterName types.String `tfsdk:"cluster_name"`
	EksArn      types.String `tfsdk:"eks_arn"`
}

func (r *AWSLogSourceEKSResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_aws_logsource_eks"
}

func (r *AWSLogSourceEKSResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Registers an EKS cluster as a log source in ExaForce CloudScout. All attributes are ForceNew (destroy + recreate on change) because no update endpoint exists.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Log source name used as the resource identifier.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Log source name. Auto-generated if omitted (eks-{accountId}-{region}-{cluster}).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"spec": schema.SingleNestedAttribute{
				Required:    true,
				Description: "EKS log source specification.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
				Attributes: map[string]schema.Attribute{
					"cluster_name": schema.StringAttribute{
						Optional:    true,
						Description: "EKS cluster name (e.g. 'prod-eks'). The provider resolves the ARN from CloudScout discovery.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"eks_arn": schema.StringAttribute{
						Optional:    true,
						Computed:    true,
						Description: "EKS cluster ARN. Set automatically from cluster_name lookup, or provide directly.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
							stringplanmodifier.UseStateForUnknown(),
						},
					},
				},
			},
		},
	}
}

func (r *AWSLogSourceEKSResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *AWSLogSourceEKSResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AWSLogSourceEKSResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var specModel EKSSpecModel
	resp.Diagnostics.Append(plan.Spec.As(ctx, &specModel, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() {
		return
	}

	eksArn := specModel.EksArn.ValueString()
	if eksArn == "" && !specModel.ClusterName.IsNull() && specModel.ClusterName.ValueString() != "" {
		cluster, err := r.client.LookupEKSClusterByName(ctx, specModel.ClusterName.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to resolve EKS cluster ARN", err.Error())
			return
		}
		eksArn = cluster.ARN
	}

	spec := client.LogSourceSpec{
		LogSourceType: "eks",
		EksArn:        eksArn,
	}

	name := plan.Name.ValueString()
	if name == "" {
		name = deriveLogSourceName(spec)
	}

	ls, err := r.client.CreateLogSource(ctx, name, spec)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create EKS log source",
			eksErrorDetail(err.Error(), spec.EksArn))
		return
	}

	if diags := r.applyAPIResponse(ctx, ls, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AWSLogSourceEKSResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AWSLogSourceEKSResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ls, status, err := r.client.GetLogSource(ctx, state.Name.ValueString())
	if err != nil {
		if status == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read EKS log source", err.Error())
		return
	}

	if diags := r.applyAPIResponse(ctx, ls, &state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *AWSLogSourceEKSResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// All attributes are ForceNew — Update is never called.
}

func (r *AWSLogSourceEKSResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AWSLogSourceEKSResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteLogSource(ctx, state.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete EKS log source", err.Error())
	}
}

func (r *AWSLogSourceEKSResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	ls, status, err := r.client.GetLogSource(ctx, req.ID)
	if err != nil {
		if status == http.StatusNotFound {
			resp.Diagnostics.AddError("Log source not found", fmt.Sprintf("No EKS log source with name %q", req.ID))
			return
		}
		resp.Diagnostics.AddError("Failed to import EKS log source", err.Error())
		return
	}

	var state AWSLogSourceEKSResourceModel
	if diags := r.applyAPIResponse(ctx, ls, &state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// eksErrorDetail enriches API error messages with actionable remediation steps.
func eksErrorDetail(apiErr, eksArn string) string {
	region, clusterName := "", ""
	parts := strings.Split(eksArn, ":")
	if len(parts) >= 6 {
		region = parts[3]
		clusterName = strings.TrimPrefix(parts[5], "cluster/")
	}

	switch {
	case strings.Contains(apiErr, "not configured for audit logging"):
		fix := ""
		if clusterName != "" {
			fix = fmt.Sprintf(`
Fix -- update your existing Terraform configuration to enable logging for the EKS cluster:

  resource "aws_eks_cluster" "%s" {
    name     = "%s"
    role_arn = aws_iam_role.eks_cluster.arn
    enabled_cluster_log_types = ["api", "audit", "authenticator", "controllerManager", "scheduler"]
  }

Then run: terraform apply (for the aws_eks_cluster resource first)`, clusterName, clusterName)
		}
		return apiErr + fix

	case strings.Contains(apiErr, "doesn't have configured subscription filter"):
		destArn := ""
		logGroup := fmt.Sprintf("/aws/eks/%s/cluster", clusterName)
		if idx := strings.LastIndex(apiErr, "subscription filter "); idx != -1 {
			destArn = strings.TrimSpace(apiErr[idx+len("subscription filter "):])
			destArn = strings.Trim(destArn, `"`)
		}
		fix := ""
		if region != "" && clusterName != "" && destArn != "" {
			filterName := fmt.Sprintf("exaforce-%s-filter-%s", clusterName, region)
			fix = fmt.Sprintf(`
Fix -- add to your Terraform (in the customer's AWS account):

  resource "aws_cloudwatch_log_subscription_filter" "exaforce_%s" {
    name            = "%s"
    log_group_name  = "%s"
    filter_pattern  = "{ $.kind = \"*\" }"
    destination_arn = "%s"
  }

Then re-run: terraform apply`,
				strings.ReplaceAll(clusterName, "-", "_"),
				filterName, logGroup, destArn)
		}
		return apiErr + fix
	}

	return apiErr
}

// deriveLogSourceName generates a name from the log source spec when the user
// does not provide one. Pattern: eks-{account}-{region}-{cluster}, max 63 chars.
func deriveLogSourceName(spec client.LogSourceSpec) string {
	var name string
	switch spec.LogSourceType {
	case "eks":
		parts := strings.Split(spec.EksArn, ":")
		if len(parts) >= 6 {
			region := parts[3]
			account := parts[4]
			cluster := strings.TrimPrefix(parts[5], "cluster/")
			name = fmt.Sprintf("eks-%s-%s-%s", account, region, cluster)
		}
	}
	if name == "" {
		name = spec.LogSourceType + "-logsource"
	}
	if len(name) > 63 {
		name = name[:63]
	}
	name = strings.TrimRight(name, "-")
	return name
}

func (r *AWSLogSourceEKSResource) applyAPIResponse(ctx context.Context, ls *client.LogSource, model *AWSLogSourceEKSResourceModel) diag.Diagnostics {
	model.ID = types.StringValue(ls.Name())
	model.Name = types.StringValue(ls.Name())

	clusterName := types.StringNull()
	if !model.Spec.IsNull() && !model.Spec.IsUnknown() {
		var existing EKSSpecModel
		if diags := model.Spec.As(ctx, &existing, basetypes.ObjectAsOptions{}); !diags.HasError() {
			clusterName = existing.ClusterName
		}
	}

	specObj, diags := types.ObjectValue(eksSpecAttrTypes, map[string]attr.Value{
		"cluster_name": clusterName,
		"eks_arn":      types.StringValue(ls.Spec.EksArn),
	})
	model.Spec = specObj
	return diags
}
