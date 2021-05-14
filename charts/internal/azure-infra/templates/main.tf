provider "azurerm" {
  subscription_id = var.SUBSCRIPTION_ID
  tenant_id       = var.TENANT_ID
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

{{- range $i, $subnet := .Values.networks.subnets }}
{{  $index := $i }}
{{- $workers := "workers" }}
{{- $subnetName := printf "%s-nodes" $.Values.clusterName }}
{{- $subnetOutput := printf "%s" $.Values.outputKeys.subnetName }}
{{- if ne $index 0 }}
{{- $workers = printf "%s-z%d" $workers $index }}
{{- $subnetName = printf "%s-z%d" $subnetName $index }}
{{- $subnetOutput = printf "%s-z%d" $subnetOutput $index }}
{{- end }}

#===============================================
#= Subnet
#===============================================

# Subnet
resource "azurerm_subnet" "{{ $workers }}" {
  name                      = {{ $subnetName }}
  {{ if $.Values.create.vnet -}}
  virtual_network_name      = azurerm_virtual_network.vnet.name
  resource_group_name       = azurerm_virtual_network.vnet.resource_group_name
  {{- else -}}
  virtual_network_name      = data.azurerm_virtual_network.vnet.name
  resource_group_name       = data.azurerm_virtual_network.vnet.resource_group_name
  {{- end }}
  address_prefixes          = ["{{ required "networks.worker is required" $subnet.cidr }}"]
  service_endpoints         = [{{ range $index, $serviceEndpoint := $subnet.serviceEndpoints }}{{ if $index }},{{ end }}"{{$serviceEndpoint}}"{{end}}]
}

resource "azurerm_subnet_route_table_association" "{{ $workers }}-rt-subnet-association" {
  subnet_id      = azurerm_subnet.{{ $workers }}.id
  route_table_id = azurerm_route_table.workers.id
}

resource "azurerm_subnet_network_security_group_association" "{{ $workers }}-nsg-subnet-association" {
  subnet_id                 = azurerm_subnet.{{ $workers }}.id
  network_security_group_id = azurerm_network_security_group.{{ $workers }}.id
}

output "{{ $subnetOutput }}" {
  value = azurerm_subnet.{{ $workers }}.name
}

{{- if hasKey $subnet "natGateway" }}
{{- if $subnet.natGateway }}

#===============================================
#= NAT Gateway
#===============================================

{{- $natName := printf "nat-z%d" $index }}
resource "azurerm_nat_gateway" "{{ $natName }}" {
  name                    = "{{ required "clusterName is required" $.Values.clusterName }}-nat-gateway-{{ $index }}"
  location                = "{{ required "azure.region is required" $.Values.azure.region }}"
  resource_group_name     = {{ template "resource-group-reference" $ }}
  sku_name                = "Standard"
  {{ if hasKey $subnet.natGateway "idleConnectionTimeoutMinutes" -}}
  idle_timeout_in_minutes = {{ $subnet.natGateway.idleConnectionTimeoutMinutes }}
  {{- end }}
  {{ if hasKey $subnet.natGateway "zone" -}}
  zones = [{{ $subnet.natGateway.zone | quote }}]
  {{- end }}
  {{ if $subnet.natGateway.migrateNatGatewayToIPAssociation -}}
  # TODO(natipmigration) This can be removed in future versions when the ip migration has been completed.
  public_ip_address_ids   = []
  {{- end }}
}
resource "azurerm_subnet_nat_gateway_association" "nat-worker-subnet-association" {
  subnet_id      = azurerm_subnet.workers.id
  nat_gateway_id = azurerm_nat_gateway.nat.id
}

{{ if and (hasKey $subnet.natGateway "ipAddresses") (hasKey $subnet.natGateway "zone") -}}
{{ range $ipIndex, $ip := .natGateway.ipAddresses -}}
data "azurerm_public_ip" "{{ $natName }}-ip-user-provided-{{ $ipIndex }}" {
  name                = "{{ $ip.name }}"
  resource_group_name = "{{ $ip.resourceGroup }}"
}
resource "azurerm_nat_gateway_public_ip_association" "{{ $natName }}-ip-user-provided-{{ $ipIndex }}-association" {
  nat_gateway_id       = azurerm_nat_gateway.{{ $natName }}.id
  public_ip_address_id = data.azurerm_public_ip.{{ $natName }}-ip-user-provided-{{ $ipIndex }}.id
}
{{- end }}
{{- else -}}
resource "azurerm_public_ip" "natip" {
  name                = "{{ required "clusterName is required" $.Values.clusterName }}-{{ $natName }}-ip"
  location            = "{{ required "azure.region is required" $.Values.azure.region }}"
  resource_group_name = {{ template "resource-group-reference" $ }}
  allocation_method   = "Static"
  sku                 = "Standard"
  {{ if hasKey $subnet.natGateway "zone" -}}
  zones = [{{ .Values.natGateway.zone | quote }}]
  {{- end }}
}
resource "azurerm_nat_gateway_public_ip_association" "natip-association" {
  nat_gateway_id       = azurerm_nat_gateway.nat.id
  public_ip_address_id = azurerm_public_ip.natip.id
}
{{- end }}

{{- end }}
{{- end }}

{{- end }}

#===============================================
#= Internal Subnet
#===============================================

{{- if hasKey .Values.networks "internal" }}
{{- if .Values.networks.internal }}
resource "azurerm_subnet" "internal" {
  name                      = {{ $.Values.clusterName }}-internal
  {{ if .Values.create.vnet -}}
  virtual_network_name      = azurerm_virtual_network.vnet.name
  resource_group_name       = azurerm_virtual_network.vnet.resource_group_name
  {{- else -}}
  virtual_network_name      = data.azurerm_virtual_network.vnet.name
  resource_group_name       = data.azurerm_virtual_network.vnet.resource_group_name
  {{- end }}
  address_prefixes          = ["{{ required "networks.internal is required" .Values.networks.internal }}"]
}

resource "azurerm_subnet_route_table_association" "internal-rt-subnet-association" {
  subnet_id      = azurerm_subnet.internal.id
  route_table_id = azurerm_route_table.internal.id
}

resource "azurerm_subnet_network_security_group_association" "internal-nsg-subnet-association" {
  subnet_id                 = azurerm_subnet.internal.id
  network_security_group_id = azurerm_network_security_group.internal.id
}

output "{{ .Values.outputKeys.subnetName }}-internal" {
  value = azurerm_subnet.internal.name
}
{{- end }}
{{- end }}

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
