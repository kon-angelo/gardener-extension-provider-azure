provider "azurerm" {
  subscription_id = "{{ required "azure.subscriptionID is required" .Values.azure.subscriptionID }}"
  tenant_id       = "{{ required "azure.tenantID is required" .Values.azure.tenantID }}"
  client_id       = var.CLIENT_ID
  client_secret   = var.CLIENT_SECRET

  features {}
}

#===============================================
#= Resource Group
#===============================================
{{ if .Values.create.resourceGroup -}}
resource "azurerm_resource_group" "rg" {
  name     = "{{ required "resourceGroup.name is required" .Values.resourceGroup.name }}"
  location = "{{ required "azure.region is required" .Values.azure.region }}"
}
{{- else -}}
data "azurerm_resource_group" "rg" {
  name     = "{{ required "resourceGroup.name is required" .Values.resourceGroup.name }}"
}
{{- end}}

#===============================================
#= VNet, Subnets, Route Table, Security Groups
#===============================================

# VNet
{{ if .Values.create.vnet -}}
resource "azurerm_virtual_network" "vnet" {
  name                = "{{ required "resourceGroup.vnet.name is required" .Values.resourceGroup.vnet.name }}"
  resource_group_name = {{ template "resource-group-reference" . }}
  location            = "{{ required "azure.region is required" .Values.azure.region }}"
  address_space       = ["{{ required "resourceGroup.vnet.cidr is required" .Values.resourceGroup.vnet.cidr }}"]
}
{{- else -}}
data "azurerm_virtual_network" "vnet" {
  name                = "{{ required "resourceGroup.vnet.name is required" .Values.resourceGroup.vnet.name }}"
  resource_group_name = "{{ required "resourceGroup.vnet.resourceGroup is required" .Values.resourceGroup.vnet.resourceGroup }}"
}
{{- end }}

# RouteTable
resource "azurerm_route_table" "workers" {
  name                = "worker_route_table"
  location            = "{{ required "azure.region is required" .Values.azure.region }}"
  resource_group_name = {{ template "resource-group-reference" . }}
}

# SecurityGroup
resource "azurerm_network_security_group" "workers" {
  name                = "{{ required "clusterName is required" .Values.clusterName }}-workers"
  location            = "{{ required "azure.region is required" .Values.azure.region }}"
  resource_group_name = {{ template "resource-group-reference" . }}
}

###################### OLD STUFF ##########################
###################### OLD STUFF ##########################
###################### OLD STUFF ##########################
# Subnet
{{ if .Values.networks.worker -}}
resource "azurerm_subnet" "workers" {
  name                      = "{{ required "clusterName is required" .Values.clusterName }}-nodes"
  {{ if .Values.create.vnet -}}
  virtual_network_name      = azurerm_virtual_network.vnet.name
  resource_group_name       = azurerm_virtual_network.vnet.resource_group_name
  {{- else -}}
  virtual_network_name      = data.azurerm_virtual_network.vnet.name
  resource_group_name       = data.azurerm_virtual_network.vnet.resource_group_name
  {{- end }}
  address_prefixes          = ["{{ required "networks.worker is required" .Values.networks.worker }}"]
  service_endpoints         = [{{range $index, $serviceEndpoint := .Values.resourceGroup.subnet.serviceEndpoints}}{{if $index}},{{end}}"{{$serviceEndpoint}}"{{end}}]
}

resource "azurerm_subnet_route_table_association" "workers-rt-subnet-association" {
  subnet_id      = azurerm_subnet.workers.id
  route_table_id = azurerm_route_table.workers.id
}

resource "azurerm_subnet_network_security_group_association" "workers-nsg-subnet-association" {
  subnet_id                 = azurerm_subnet.workers.id
  network_security_group_id = azurerm_network_security_group.workers.id
}

output "{{ .Values.outputKeys.subnetName }}" {
  value = azurerm_subnet.workers.name
}

{{ if .Values.create.natGateway -}}
#===============================================
#= NAT Gateway
#===============================================

resource "azurerm_nat_gateway" "nat" {
  name                    = "{{ required "clusterName is required" .Values.clusterName }}-nat-gateway"
  location                = "{{ required "azure.region is required" .Values.azure.region }}"
  resource_group_name     = {{ template "resource-group-reference" . }}
  sku_name                = "Standard"
  {{ if .Values.natGateway -}}
  {{ if hasKey .Values.natGateway "idleConnectionTimeoutMinutes" -}}
  idle_timeout_in_minutes = {{ .Values.natGateway.idleConnectionTimeoutMinutes }}
  {{- end }}
  {{ if hasKey .Values.natGateway "zone" -}}
  zones = [{{ .Values.natGateway.zone | quote }}]
  {{- end }}
  {{ if .Values.natGateway.migrateNatGatewayToIPAssociation -}}
  # TODO(natipmigration) This can be removed in future versions when the ip migration has been completed.
  public_ip_address_ids   = []
  {{- end }}
  {{- end }}
}
resource "azurerm_subnet_nat_gateway_association" "nat-worker-subnet-association" {
  subnet_id      = azurerm_subnet.workers.id
  nat_gateway_id = azurerm_nat_gateway.nat.id
}

{{ if .Values.natGateway -}}
{{ if and (hasKey .Values.natGateway "ipAddresses") (hasKey .Values.natGateway "zone") -}}
{{ template "natgateway-user-provided-public-ips" . }}
{{- else -}}
{{ template "natgateway-managed-ip" . }}
{{- end }}
{{- else -}}
{{ template "natgateway-managed-ip" . }}
{{- end }}
{{- end }}
{{- end }}

###################### NEW STUFF ##########################
###################### NEW STUFF ##########################
###################### NEW STUFF ##########################
{{- range $zIndex, $zone := .Values.networks.zones }}

# Subnet

{{ $subnet := printf "nodes-z-%d" $zIndex }}
resource "azurerm_subnet" "{{ $subnet }}" {
  name                      = "{{ required "clusterName is required" $.Values.clusterName }}-z-{{ required "zone is required" $zIndex }}"
  {{ if $.Values.create.vnet -}}
  virtual_network_name      = azurerm_virtual_network.vnet.name
  resource_group_name       = azurerm_virtual_network.vnet.resource_group_name
  {{- else -}}
  virtual_network_name      = data.azurerm_virtual_network.vnet.name
  resource_group_name       = data.azurerm_virtual_network.vnet.resource_group_name
  natGatewayConfig{{- end }}
  address_prefixes          = ["{{ required "networks.worker is required" $zone.cidr }}"]
  service_endpoints         = [{{range $index, $serviceEndpoint := $zone.serviceEndpoints}}{{if $index}},{{end}}"{{$serviceEndpoint}}"{{end}}]
}

resource "azurerm_subnet_route_table_association" "{{ $subnet }}-rt-subnet-association" {
  subnet_id      = azurerm_subnet.{{ $subnet }}.id
  route_table_id = azurerm_route_table.workers.id
}

resource "azurerm_subnet_network_security_group_association" "{{ $subnet }}-nsg-subnet-association" {
  subnet_id      = azurerm_subnet.{{ $subnet }}.id
  network_security_group_id = azurerm_network_security_group.workers.id
}

output "{{ $.Values.outputKeys.subnetNamePrefix }}{{ $zIndex }}" {
  value = azurerm_subnet.{{ $subnet }}.name
}

{{- if hasKey $zone "natGateway" }}
#===============================================
#= NAT Gateway
#===============================================

{{ $nat := printf "nat-z-%d" $zIndex -}}
resource "azurerm_nat_gateway" "{{ $nat }}" {
  name                    = "{{ required "clusterName is required" $.Values.clusterName }}-z{{ required "zone name is required" $zIndex }}-nat-gateway"
  location                = "{{ required "azure.region is required" $.Values.azure.region }}"
  sku_name                = "Standard"
  resource_group_name     = {{ template "resource-group-reference" $ }}
  {{- if $zone.natGateway }}
  {{ if hasKey $zone.natGateway "idleConnectionTimeoutMinutes" -}}
  idle_timeout_in_minutes = {{ $zone.natGateway.idleConnectionTimeoutMinutes }}
  {{- end }}
  {{- if and (hasKey $zone.natGateway "zone") $zone.natGateway.zone }}
  zones                   = ["{{ $zone.natGateway.zone }}"]
  {{- else }}
  zones                   = ["{{ $zone.name }}"]
  {{- end }}
  {{- end }}
}

resource "azurerm_subnet_nat_gateway_association" "{{ $nat }}-subnet-association" {
  subnet_id                 = azurerm_subnet.{{ $subnet }}.id
  nat_gateway_id            = azurerm_nat_gateway.{{ $nat }}.id
}

resource "azurerm_public_ip" "{{ $nat }}-pip" {
  name                    = "{{ required "clusterName is required" $.Values.clusterName }}-nat-ip-z{{ $zIndex }}"
  location                = "{{ required "azure.region is required" $.Values.azure.region }}"
  resource_group_name     = {{ template "resource-group-reference" $ }}
  allocation_method       = "Static"
  sku                     = "Standard"
  {{- if $zone.natGateway }}
  {{- if and (hasKey $zone.natGateway "zone") $zone.natGateway.zone }}
  zones                   = ["{{ $zone.natGateway.zone }}"]
  {{- else }}
  zones                   = ["{{ $zone.name }}"]
  {{- end }}
  {{- end }}
}
resource "azurerm_nat_gateway_public_ip_association" "{{ $nat }}-pip-association" {
  nat_gateway_id            = azurerm_nat_gateway.{{ $nat }}.id
  public_ip_address_id      = azurerm_public_ip.{{ $nat }}-pip.id
}
{{- end -}}
{{- end -}}

{{ if .Values.identity -}}
#===============================================
#= Identity
#===============================================

data "azurerm_user_assigned_identity" "identity" {
  name                = "{{ required "identity.name is required" .Values.identity.name }}"
  resource_group_name = "{{ required "identity.resourceGroup is required" .Values.identity.resourceGroup }}"
}
{{- end }}

{{ if .Values.create.availabilitySet -}}
#===============================================
#= Availability Set
#===============================================

resource "azurerm_availability_set" "workers" {
  name                         = "{{ required "clusterName is required" .Values.clusterName }}-avset-workers"
  location                     = "{{ required "azure.region is required" .Values.azure.region }}"
  resource_group_name          = {{ template "resource-group-reference" . }}
  platform_update_domain_count = "{{ required "azure.countUpdateDomains is required" .Values.azure.countUpdateDomains }}"
  platform_fault_domain_count  = "{{ required "azure.countFaultDomains is required" .Values.azure.countFaultDomains }}"
  managed                      = true
}
{{- end}}

#===============================================
//= Output variables
#===============================================

output "{{ .Values.outputKeys.resourceGroupName }}" {
  value = {{ template "resource-group-reference" . }}
}

{{ if .Values.create.vnet -}}
output "{{ .Values.outputKeys.vnetName }}" {
  value = azurerm_virtual_network.vnet.name
}
{{- else -}}
output "{{ .Values.outputKeys.vnetName }}" {
  value = data.azurerm_virtual_network.vnet.name
}

output "{{ .Values.outputKeys.vnetResourceGroup }}" {
  value = data.azurerm_virtual_network.vnet.resource_group_name
}
{{- end}}

output "{{ .Values.outputKeys.routeTableName }}" {
  value = azurerm_route_table.workers.name
}

output "{{ .Values.outputKeys.securityGroupName }}" {
  value = azurerm_network_security_group.workers.name
}

{{ if .Values.create.availabilitySet -}}
output "{{ .Values.outputKeys.availabilitySetID }}" {
  value = azurerm_availability_set.workers.id
}

output "{{ .Values.outputKeys.availabilitySetName }}" {
  value = azurerm_availability_set.workers.name
}

output "{{ .Values.outputKeys.countFaultDomains }}" {
  value = azurerm_availability_set.workers.platform_fault_domain_count
}

output "{{ .Values.outputKeys.countUpdateDomains }}" {
  value = azurerm_availability_set.workers.platform_update_domain_count
}
{{- end}}
{{ if .Values.identity -}}
output "{{ .Values.outputKeys.identityID }}" {
  value = data.azurerm_user_assigned_identity.identity.id
}

output "{{ .Values.outputKeys.identityClientID }}" {
  value = data.azurerm_user_assigned_identity.identity.client_id
}
{{- end }}
