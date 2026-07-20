package provider

import (
	"context"
	"fmt"

	"github.com/ExaForce/terraform-provider-exaforceio/internal/provider/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &AWSEKSClustersDataSource{}

type AWSEKSClustersDataSource struct {
	client *client.Client
}

func NewAWSEKSClustersDataSource() datasource.DataSource {
	return &AWSEKSClustersDataSource{}
}

type AWSEKSClustersDataSourceModel struct {
	Clusters types.List `tfsdk:"clusters"`
}

var eksClusterAttrTypes = map[string]attr.Type{
	"name":            types.StringType,
	"arn":             types.StringType,
	"account_id":      types.StringType,
	"region":          types.StringType,
	"exabot_role_arn": types.StringType,
	"exabot_sqs_url":  types.StringType,
	"bucket_id":       types.StringType,
}

func (d *AWSEKSClustersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_aws_eks_clusters"
}

func (d *AWSEKSClustersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists all EKS clusters discovered by ExaForce CloudScout for the configured project.",
		Attributes: map[string]schema.Attribute{
			"clusters": schema.ListNestedAttribute{
				Computed:    true,
				Description: "EKS clusters discovered by CloudScout.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":            schema.StringAttribute{Computed: true, Description: "EKS cluster name."},
						"arn":             schema.StringAttribute{Computed: true, Description: "EKS cluster ARN."},
						"account_id":      schema.StringAttribute{Computed: true, Description: "AWS account ID."},
						"region":          schema.StringAttribute{Computed: true, Description: "AWS region."},
						"exabot_role_arn": schema.StringAttribute{Computed: true, Description: "IAM role ARN for the ExaForce agent (exabot-k8s)."},
						"exabot_sqs_url":  schema.StringAttribute{Computed: true, Description: "SQS queue URL for the ExaForce agent (exabot-k8s)."},
						"bucket_id":       schema.StringAttribute{Computed: true, Description: "S3 config bucket ID for the ExaForce agent (exabot-k8s)."},
					},
				},
			},
		},
	}
}

func (d *AWSEKSClustersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *AWSEKSClustersDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	clusters, err := d.client.ListEKSClusters(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to list EKS clusters", err.Error())
		return
	}

	items := make([]attr.Value, 0, len(clusters))
	for _, c := range clusters {
		obj, diags := types.ObjectValue(eksClusterAttrTypes, map[string]attr.Value{
			"name":            types.StringValue(c.Name),
			"arn":             types.StringValue(c.ARN),
			"account_id":      types.StringValue(c.AccountID),
			"region":          types.StringValue(c.Region),
			"exabot_role_arn": types.StringValue(c.ExabotRoleArn),
			"exabot_sqs_url":  types.StringValue(c.ExabotSqsURL),
			"bucket_id":       types.StringValue(c.BucketID),
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		items = append(items, obj)
	}

	list, diags := types.ListValue(types.ObjectType{AttrTypes: eksClusterAttrTypes}, items)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &AWSEKSClustersDataSourceModel{Clusters: list})...)
}
