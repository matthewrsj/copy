"""
This generates the config file for Netplan which will assign the appropriate IP address based on MAC address.
Input: macs.yaml, formatted with first level being aisle, second being tower: MAC
Output: 01-static-ethernet-from-mac.yaml, Move this file to /etc/netplan/ and delete any existing yaml files. 
"""
import yaml

ip_prefix = "10.179.205"
tc_ip_offset = 63 # TC addresses start at .64
aisle_ip_offset = 10 # reserved TC addresses are 10 per aisle
gateway = "10.179.205.1"
nameserver1 = "10.178.8.53"
nameserver2 = "10.33.169.12"
net = {"network": {"version": 2, "renderer": "networkd"}}
route_to = "192.168.1.0"
route_via = "10.179.205"
via_for_aisle_start = 44 # route_via starts at .45

with open("macs.yaml", "r") as stream:
    macs = yaml.safe_load(stream)
ethernets = {}
for aisle in macs:
    for tower in macs[aisle]:
        mac = macs[aisle][tower]
        ip = tc_ip_offset + aisle * aisle_ip_offset + tower
        via_for_aisle = via_for_aisle_start + aisle
        ethernets[f"a{aisle}t{tower}"] = {
            "match": {"macaddress": mac},
            "addresses": [f"{ip_prefix}.{ip}/24"],
            "gateway4": gateway,
            "nameservers": {"addresses": [nameserver1, nameserver2]},
            "routes": [{"to": f"{route_to}/24", "via": f"{route_via}.{via_for_aisle}"}]
        }
net["network"]["ethernets"] = ethernets
print(yaml.dump(net, default_flow_style=False))
with open(r"01-static-ethernet-from-mac.yaml", "w") as file:
    documents = yaml.dump(net, file, default_flow_style=False)
