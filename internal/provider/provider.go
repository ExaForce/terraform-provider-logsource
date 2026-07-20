package provider

import (
	"context"
	"os"

	"github.com/ExaForce/terraform-provider-logsource/internal/provider/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &ExaforceProvider{}
var _ provider.ProviderWithFunctions = &ExaforceProvider{}

type ExaforceProvider struct {
	version string
}

type ExaforceProviderModel struct {
	Endpoint    types.String `tfsdk:"endpoint"`
	APIToken    types.String `tfsdk:"api_token"`
	IDToken     types.String `tfsdk:"id_token"`
	AccessToken types.String `tfsdk:"access_token"`
	Session     types.String `tfsdk:"session"`
	Project     types.String `tfsdk:"project"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &ExaforceProvider{version: version}
	}
}

func (p *ExaforceProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "exaforce"
	resp.Version = p.version
}

func (p *ExaforceProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for ExaForce security platform. Manages AWS log source registration via CloudScout API.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Description: "ExaForce API base URL, e.g. https://<tenant>.us.app.exaforce.io. Env: EXAFORCE_ENDPOINT.",
				Optional:    true,
			},
			"api_token": schema.StringAttribute{
				Description: "ExaForce API token (X-EXF-API-TOKEN). Env: EXAFORCE_API_TOKEN. Preferred auth — use this once the auth PR lands.",
				Optional:    true,
				Sensitive:   true,
			},
			"id_token": schema.StringAttribute{
				Description: "Temporary: X-EXF-ID-TOKEN cookie value. Env: EXAFORCE_ID_TOKEN. Used as fallback when api_token is not set.",
				Optional:    true,
				Sensitive:   true,
			},
			"access_token": schema.StringAttribute{
				Description: "Temporary: X-EXF-ACCESS-TOKEN cookie value. Env: EXAFORCE_ACCESS_TOKEN. Used as fallback when api_token is not set.",
				Optional:    true,
				Sensitive:   true,
			},
			"session": schema.StringAttribute{
				Description: "Temporary: session cookie value. Env: EXAFORCE_SESSION. Required alongside id_token/access_token.",
				Optional:    true,
				Sensitive:   true,
			},
			"project": schema.StringAttribute{
				Description: "ExaForce project name (default: 'default'). Env: EXAFORCE_PROJECT.",
				Optional:    true,
			},
		},
	}
}

func (p *ExaforceProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config ExaforceProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := orEnv(config.Endpoint, "EXAFORCE_ENDPOINT")
	apiToken := orEnv(config.APIToken, "EXAFORCE_API_TOKEN")
	idToken := orEnv(config.IDToken, "EXAFORCE_ID_TOKEN")
	accessToken := orEnv(config.AccessToken, "EXAFORCE_ACCESS_TOKEN")
	session := orEnv(config.Session, "EXAFORCE_SESSION")
	project := orEnv(config.Project, "EXAFORCE_PROJECT")

	if project == "" {
		project = "default"
	}
	if endpoint == "" {
		resp.Diagnostics.AddError("Missing endpoint", "Set endpoint in provider config or EXAFORCE_ENDPOINT env var.")
		return
	}
	if apiToken == "" && (idToken == "" || accessToken == "") {
		resp.Diagnostics.AddError("Missing auth",
			"Provide either api_token (EXAFORCE_API_TOKEN) or both id_token + access_token (EXAFORCE_ID_TOKEN + EXAFORCE_ACCESS_TOKEN).")
		return
	}

	c := client.New(endpoint, apiToken, idToken, accessToken, session, project)
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *ExaforceProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewAWSLogSourceEKSResource,
	}
}

func (p *ExaforceProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewAWSLogSourcesDataSource,
		NewAWSEKSClustersDataSource,
	}
}

func (p *ExaforceProvider) Functions(_ context.Context) []func() function.Function {
	return nil
}

func orEnv(val types.String, envKey string) string {
	if !val.IsNull() && !val.IsUnknown() && val.ValueString() != "" {
		return val.ValueString()
	}
	return os.Getenv(envKey)
}
