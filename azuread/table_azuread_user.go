package azuread

import (
	"context"
	"fmt"
	"strings"

	"github.com/ettle/strcase"
	"github.com/manicminer/hamilton/msgraph"
	"github.com/manicminer/hamilton/odata"
	"github.com/turbot/go-kit/helpers"
	"github.com/turbot/steampipe-plugin-sdk/grpc/proto"
	"github.com/turbot/steampipe-plugin-sdk/plugin/transform"

	"github.com/turbot/steampipe-plugin-sdk/plugin"
)

//// TABLE DEFINITION

func tableAzureAdUser(_ context.Context) *plugin.Table {
	return &plugin.Table{
		Name:        "azuread_user",
		Description: "Azure AD User",
		List: &plugin.ListConfig{
			Hydrate: listAdUsers,
			KeyColumns: plugin.KeyColumnSlice{
				// Key fields
				{Name: "id", Require: plugin.Optional},
				{Name: "user_principal_name", Require: plugin.Optional},
				{Name: "filter", Require: plugin.Optional},

				// Other fields for filtering OData
				{Name: "user_type", Require: plugin.Optional},
				{Name: "account_enabled", Require: plugin.Optional, Operators: []string{"<>", "="}},
				{Name: "display_name", Require: plugin.Optional},
				{Name: "surname", Require: plugin.Optional},
			},
		},

		Columns: []*plugin.Column{
			{Name: "display_name", Type: proto.ColumnType_STRING, Description: "The name displayed in the address book for the user. This is usually the combination of the user's first name, middle initial and last name."},
			{Name: "id", Type: proto.ColumnType_STRING, Description: "The unique identifier for the user. Should be treated as an opaque identifier.", Transform: transform.FromGo()},
			{Name: "user_principal_name", Type: proto.ColumnType_STRING, Description: "Principal email of the active directory user."},
			{Name: "account_enabled", Type: proto.ColumnType_BOOL, Description: "True if the account is enabled; otherwise, false."},
			{Name: "user_type", Type: proto.ColumnType_STRING, Description: "A string value that can be used to classify user types in your directory."},
			{Name: "given_name", Type: proto.ColumnType_STRING, Description: "The given name (first name) of the user."},
			{Name: "surname", Type: proto.ColumnType_STRING, Description: "Family name or last name of the active directory user."},

			{Name: "filter", Type: proto.ColumnType_STRING, Transform: transform.FromQual("filter"), Description: "Odata query to search for resources."},

			// Other fields
			{Name: "created_date_time", Type: proto.ColumnType_TIMESTAMP, Description: "The time at which the user was created."},
			{Name: "is_management_restricted", Type: proto.ColumnType_BOOL, Description: ""},
			{Name: "mail", Type: proto.ColumnType_STRING, Description: "	The SMTP address for the user, for example, jeff@contoso.onmicrosoft.com."},
			{Name: "mail_nickname", Type: proto.ColumnType_STRING, Description: "The mail alias for the user."},
			{Name: "password_policies", Type: proto.ColumnType_STRING, Description: "Specifies password policies for the user. This value is an enumeration with one possible value being DisableStrongPassword, which allows weaker passwords than the default policy to be specified. DisablePasswordExpiration can also be specified. The two may be specified together; for example: DisablePasswordExpiration, DisableStrongPassword."},
			{Name: "refresh_tokens_valid_from_date_time", Type: proto.ColumnType_TIMESTAMP, Description: "Any refresh tokens or sessions tokens (session cookies) issued before this time are invalid, and applications will get an error when using an invalid refresh or sessions token to acquire a delegated access token (to access APIs such as Microsoft Graph)."},
			{Name: "sign_in_sessions_valid_from_date_time", Type: proto.ColumnType_TIMESTAMP, Description: "Any refresh tokens or sessions tokens (session cookies) issued before this time are invalid, and applications will get an error when using an invalid refresh or sessions token to acquire a delegated access token (to access APIs such as Microsoft Graph)."},
			{Name: "usage_location", Type: proto.ColumnType_STRING, Description: "A two letter country code (ISO standard 3166), required for users that will be assigned licenses due to legal requirement to check for availability of services in countries."},
			// {Name: "about_me", Type: proto.ColumnType_TIMESTAMP, Description: "A freeform text entry field for the user to describe themselves."},
			// {Name: "deleted_date_time", Type: proto.ColumnType_TIMESTAMP, Description: " The time at which the directory object was deleted."},
			// {Name: "is_resource_account", Type: proto.ColumnType_BOOL, Description: "Do not use – reserved for future use."},

			// Json fields
			{Name: "member_of", Type: proto.ColumnType_JSON, Description: "A list the groups and directory roles that the user is a direct member of."},
			{Name: "additional_properties", Type: proto.ColumnType_JSON, Description: "A list of unmatched properties from the message are deserialized this collection."},
			{Name: "im_addresses", Type: proto.ColumnType_JSON, Description: "The instant message voice over IP (VOIP) session initiation protocol (SIP) addresses for the user."},
			{Name: "other_mails", Type: proto.ColumnType_JSON, Description: "A list of additional email addresses for the user."},
			{Name: "password_profile", Type: proto.ColumnType_JSON, Description: "Specifies the password profile for the user. The profile contains the user’s password. This property is required when a user is created."},

			// {Name: "sign_in_activity", Type: proto.ColumnType_JSON, Description: ""},
			// {Name: "data", Type: proto.ColumnType_JSON, Description: "The unique ID that identifies an active directory user.", Transform: transform.FromValue()}, // For debugging

			// // Standard columns
			{
				Name:        "title",
				Description: ColumnDescriptionTitle,
				Type:        proto.ColumnType_STRING,
				Transform:   transform.FromField("DisplayName", "UserPrincipalName"),
			},
			{
				Name:        "tenant_id",
				Description: ColumnDescriptionTenant,
				Type:        proto.ColumnType_STRING,
				Hydrate:     getTenantId,
				Transform:   transform.FromValue(),
			},
		},
	}
}

//// LIST FUNCTION

func listAdUsers(ctx context.Context, d *plugin.QueryData, _ *plugin.HydrateData) (interface{}, error) {
	session, err := GetNewSession(ctx, d)
	if err != nil {
		return nil, err
	}

	client := msgraph.NewUsersClient(session.TenantID)
	client.BaseClient.Authorizer = session.Authorizer

	input := odata.Query{}
	if helpers.StringSliceContains(d.QueryContext.Columns, "member_of") {
		input.Expand = odata.Expand{
			Relationship: "memberOf",
			Select:       []string{"id", "displayName"},
		}
	}

	equalQuals := d.KeyColumnQuals
	quals := d.Quals

	var queryFilter string
	filter := buildQueryFilter(equalQuals)

	if quals["account_enabled"] != nil {
		// accoutEnabled doesn't support 'ne' Operator
		for _, q := range quals["account_enabled"].Quals {
			value := q.Value.GetBoolValue()
			if q.Operator == "<>" {
				if value {
					filter = append(filter, "accountEnabled eq false")
				} else {
					filter = append(filter, "accountEnabled eq true")
				}
				break
			}
		}
	}

	if equalQuals["filter"] != nil {
		queryFilter = equalQuals["filter"].GetStringValue()
	}

	if queryFilter != "" {
		input.Filter = queryFilter
	} else if len(filter) > 0 {
		input.Filter = strings.Join(filter, " and ")
	}

	// if input.Filter != "" {
	// 	plugin.Logger(ctx).Debug("Filter", "input.Filter", input.Filter)
	// }

	pagesLeft := true
	for pagesLeft {
		users, _, err := client.List(ctx, input)
		if err != nil {
			return nil, err
		}

		for _, user := range *users {
			d.StreamListItem(ctx, user)
		}
		pagesLeft = false
	}

	return nil, err
}

//// HYDRATE FUNCTIONS

func getTenantId(ctx context.Context, d *plugin.QueryData, h *plugin.HydrateData) (interface{}, error) {
	session, err := GetNewSession(ctx, d)
	if err != nil {
		return nil, err
	}

	return session.TenantID, nil
}

func buildQueryFilter(equalQuals plugin.KeyColumnEqualsQualMap) []string {
	filters := []string{}

	filterQuals := []string{
		"user_principal_name",
		"user_type",
		"id",
		"display_name",
		"surname",
	}

	if equalQuals["account_enabled"] != nil {
		filters = append(filters, fmt.Sprintf("%s eq %t", "accountEnabled", equalQuals["account_enabled"].GetBoolValue()))
	}

	for _, qual := range filterQuals {
		if equalQuals[qual] != nil {
			filters = append(filters, fmt.Sprintf("%s eq '%s'", strcase.ToCamel(qual), equalQuals[qual].GetStringValue()))
		}
	}

	return filters
}