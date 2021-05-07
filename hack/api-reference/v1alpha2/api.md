<p>Packages:</p>
<ul>
<li>
<a href="#azure.provider.extensions.gardener.cloud%2fv1alpha2">azure.provider.extensions.gardener.cloud/v1alpha2</a>
</li>
</ul>
<h2 id="azure.provider.extensions.gardener.cloud/v1alpha2">azure.provider.extensions.gardener.cloud/v1alpha2</h2>
<p>
<p>Package v1alpha2 contains the Azure provider API resources.</p>
</p>
Resource Types:
<ul><li>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.InfrastructureConfig">InfrastructureConfig</a>
</li></ul>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.InfrastructureConfig">InfrastructureConfig
</h3>
<p>
<p>InfrastructureConfig infrastructure configuration resource</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>
azure.provider.extensions.gardener.cloud/v1alpha2
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>InfrastructureConfig</code></td>
</tr>
<tr>
<td>
<code>resourceGroup</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.ResourceGroup">
ResourceGroup
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResourceGroup is azure resource group.</p>
</td>
</tr>
<tr>
<td>
<code>networks</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.NetworkConfig">
NetworkConfig
</a>
</em>
</td>
<td>
<p>Networks is the network configuration (VNet, subnets, etc.).</p>
</td>
</tr>
<tr>
<td>
<code>identity</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.IdentityConfig">
IdentityConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Identity contains configuration for the assigned managed identity.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.AvailabilitySet">AvailabilitySet
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.InfrastructureStatus">InfrastructureStatus</a>)
</p>
<p>
<p>AvailabilitySet contains information about the azure availability set</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>purpose</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.Purpose">
Purpose
</a>
</em>
</td>
<td>
<p>Purpose is the purpose of the availability set</p>
</td>
</tr>
<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID is the id of the availability set</p>
</td>
</tr>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the availability set</p>
</td>
</tr>
<tr>
<td>
<code>countFaultDomains</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>CountFaultDomains is the count of fault domains.</p>
</td>
</tr>
<tr>
<td>
<code>countUpdateDomains</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>CountUpdateDomains is the count of update domains.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.IdentityConfig">IdentityConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.InfrastructureConfig">InfrastructureConfig</a>)
</p>
<p>
<p>IdentityConfig contains configuration for the managed identity.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the identity.</p>
</td>
</tr>
<tr>
<td>
<code>resourceGroup</code></br>
<em>
string
</em>
</td>
<td>
<p>ResourceGroup is the resource group where the identity belongs to.</p>
</td>
</tr>
<tr>
<td>
<code>acrAccess</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>ACRAccess indicated if the identity should be used by the Shoot worker nodes to pull from an Azure Container Registry.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.IdentityStatus">IdentityStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.InfrastructureStatus">InfrastructureStatus</a>)
</p>
<p>
<p>IdentityStatus contains the status information of the created managed identity.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID is the Azure resource if of the identity.</p>
</td>
</tr>
<tr>
<td>
<code>clientID</code></br>
<em>
string
</em>
</td>
<td>
<p>ClientID is the client id of the identity.</p>
</td>
</tr>
<tr>
<td>
<code>acrAccess</code></br>
<em>
bool
</em>
</td>
<td>
<p>ACRAccess specifies if the identity should be used by the Shoot worker nodes to pull from an Azure Container Registry.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.InfrastructureStatus">InfrastructureStatus
</h3>
<p>
<p>InfrastructureStatus contains information about created infrastructure resources.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>networks</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.NetworkStatus">
NetworkStatus
</a>
</em>
</td>
<td>
<p>Networks is the status of the networks of the infrastructure.</p>
</td>
</tr>
<tr>
<td>
<code>resourceGroup</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.ResourceGroup">
ResourceGroup
</a>
</em>
</td>
<td>
<p>ResourceGroup is azure resource group</p>
</td>
</tr>
<tr>
<td>
<code>availabilitySets</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.AvailabilitySet">
[]AvailabilitySet
</a>
</em>
</td>
<td>
<p>AvailabilitySets is a list of created availability sets</p>
</td>
</tr>
<tr>
<td>
<code>routeTables</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.RouteTable">
[]RouteTable
</a>
</em>
</td>
<td>
<p>AvailabilitySets is a list of created route tables</p>
</td>
</tr>
<tr>
<td>
<code>securityGroups</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.SecurityGroup">
[]SecurityGroup
</a>
</em>
</td>
<td>
<p>SecurityGroups is a list of created security groups</p>
</td>
</tr>
<tr>
<td>
<code>identity</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.IdentityStatus">
IdentityStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Identity is the status of the managed identity.</p>
</td>
</tr>
<tr>
<td>
<code>zoned</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zoned indicates whether the cluster uses zones</p>
</td>
</tr>
<tr>
<td>
<code>natGatewayPublicIpMigrated</code></br>
<em>
bool
</em>
</td>
<td>
<p>NatGatewayPublicIPMigrated is an indicator if the Gardener managed public ip address is already migrated.
TODO(natipmigration) This can be removed in future versions when the ip migration has been completed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.NatGatewayConfig">NatGatewayConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.SingleSubnetZonalTopology">SingleSubnetZonalTopology</a>, 
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.Zone">Zone</a>)
</p>
<p>
<p>NatGatewayConfig contains configuration for the NAT gateway and the attached resources.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>enabled</code></br>
<em>
bool
</em>
</td>
<td>
<p>Enabled is an indicator if NAT gateway should be deployed.</p>
</td>
</tr>
<tr>
<td>
<code>idleConnectionTimeoutMinutes</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>IdleConnectionTimeoutMinutes specifies the idle connection timeout limit for NAT gateway in minutes.</p>
</td>
</tr>
<tr>
<td>
<code>ipAddresses</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.PublicIPReference">
[]PublicIPReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>IPAddresses is a list of ip addresses which should be assigned to the NAT gateway.</p>
</td>
</tr>
<tr>
<td>
<code>Zone</code></br>
<em>
int32
</em>
</td>
<td>
<p>Zone specifies the zone in which the NAT gateway should be deployed to.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.NetworkConfig">NetworkConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.InfrastructureConfig">InfrastructureConfig</a>)
</p>
<p>
<p>NetworkConfig holds information about the Kubernetes and infrastructure networks.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>vnet</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.VNet">
VNet
</a>
</em>
</td>
<td>
<p>VNet indicates whether to use an existing VNet or create a new one.</p>
</td>
</tr>
<tr>
<td>
<code>Topology</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.Topology">
Topology
</a>
</em>
</td>
<td>
<p>
(Members of <code>Topology</code> are embedded into this type.)
</p>
<p>Topology</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.NetworkStatus">NetworkStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.InfrastructureStatus">InfrastructureStatus</a>)
</p>
<p>
<p>NetworkStatus is the current status of the infrastructure networks.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>vnet</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.VNetStatus">
VNetStatus
</a>
</em>
</td>
<td>
<p>VNetStatus states the name of the infrastructure VNet.</p>
</td>
</tr>
<tr>
<td>
<code>subnets</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.Subnet">
[]Subnet
</a>
</em>
</td>
<td>
<p>Subnets are the subnets that have been created.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.PublicIPReference">PublicIPReference
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.NatGatewayConfig">NatGatewayConfig</a>)
</p>
<p>
<p>PublicIPReference contains information about a public ip.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the public ip.</p>
</td>
</tr>
<tr>
<td>
<code>resourceGroup</code></br>
<em>
string
</em>
</td>
<td>
<p>ResourceGroup is the name of the resource group where the public ip is assigned to.</p>
</td>
</tr>
<tr>
<td>
<code>zone</code></br>
<em>
int32
</em>
</td>
<td>
<p>Zone is the zone in which the public ip is deployed to.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.Purpose">Purpose
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.AvailabilitySet">AvailabilitySet</a>, 
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.RouteTable">RouteTable</a>, 
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.SecurityGroup">SecurityGroup</a>, 
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.Subnet">Subnet</a>)
</p>
<p>
<p>Purpose is a purpose of a subnet.</p>
</p>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.RegionalTopology">RegionalTopology
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.Topology">Topology</a>)
</p>
<p>
<p>RegionalTopology contains the configuration for a network setup that is not zone based.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>cidr</code></br>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>serviceEndpoints</code></br>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.ResourceGroup">ResourceGroup
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.InfrastructureConfig">InfrastructureConfig</a>, 
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.InfrastructureStatus">InfrastructureStatus</a>)
</p>
<p>
<p>ResourceGroup is azure resource group</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the resource group</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.RouteTable">RouteTable
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.InfrastructureStatus">InfrastructureStatus</a>)
</p>
<p>
<p>RouteTable is the azure route table</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>purpose</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.Purpose">
Purpose
</a>
</em>
</td>
<td>
<p>Purpose is the purpose of the route table</p>
</td>
</tr>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the route table</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.SecurityGroup">SecurityGroup
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.InfrastructureStatus">InfrastructureStatus</a>)
</p>
<p>
<p>SecurityGroup contains information about the security group</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>purpose</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.Purpose">
Purpose
</a>
</em>
</td>
<td>
<p>Purpose is the purpose of the security group</p>
</td>
</tr>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the security group</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.SingleSubnetZonalTopology">SingleSubnetZonalTopology
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.Topology">Topology</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>cidr</code></br>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>serviceEndpoints</code></br>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>natGateway</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.NatGatewayConfig">
NatGatewayConfig
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.Subnet">Subnet
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.NetworkStatus">NetworkStatus</a>)
</p>
<p>
<p>Subnet is a subnet that was created.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the subnet.</p>
</td>
</tr>
<tr>
<td>
<code>purpose</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.Purpose">
Purpose
</a>
</em>
</td>
<td>
<p>Purpose is the purpose for which the subnet was created.</p>
</td>
</tr>
<tr>
<td>
<code>zone</code></br>
<em>
int32
</em>
</td>
<td>
<p>Zone</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.Topology">Topology
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.NetworkConfig">NetworkConfig</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>regional</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.RegionalTopology">
RegionalTopology
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>singleSubnet</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.SingleSubnetZonalTopology">
SingleSubnetZonalTopology
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>zonal</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.ZonalTopology">
ZonalTopology
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.VNet">VNet
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.NetworkConfig">NetworkConfig</a>)
</p>
<p>
<p>VNet contains information about the VNet and some related resources.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Name is the name of an existing vNet which should be used.</p>
</td>
</tr>
<tr>
<td>
<code>resourceGroup</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResourceGroup is the resource group where the existing vNet blongs to.</p>
</td>
</tr>
<tr>
<td>
<code>cidr</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>CIDR is the VNet CIDR</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.VNetStatus">VNetStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.NetworkStatus">NetworkStatus</a>)
</p>
<p>
<p>VNetStatus contains the VNet name.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the VNet name.</p>
</td>
</tr>
<tr>
<td>
<code>resourceGroup</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResourceGroup is the resource group where the existing vNet belongs to.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.ZonalTopology">ZonalTopology
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.Topology">Topology</a>)
</p>
<p>
<p>ZonalTopology contains the configuration for a network setup that is zone based. A ZonalTopology contains multiple subnets - one for each zone defined.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>zones</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.Zone">
[]Zone
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>internal</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>InternalCIDR</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha2.Zone">Zone
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.ZonalTopology">ZonalTopology</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>cidr</code></br>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>serviceEndpoints</code></br>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>natGateway</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha2.NatGatewayConfig">
NatGatewayConfig
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <a href="https://github.com/ahmetb/gen-crd-api-reference-docs">gen-crd-api-reference-docs</a>
</em></p>
