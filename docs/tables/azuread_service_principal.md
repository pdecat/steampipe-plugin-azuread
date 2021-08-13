# Table: azuread_service_principal

Azure Active Directory (Azure AD) lets you use service principal to manage access to your cloud-based resources.

## Examples

### Basic info

```sql
select
  display_name,
  id,
  app_id
from
  azuread_service_principal;
```

### List disabled service principals

```sql
select
  display_name,
  id
from
  azuread_service_principal
where
  not account_enabled;
```