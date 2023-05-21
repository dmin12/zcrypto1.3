package verifier

import (
	"context"
	"encoding/hex"
	"net/http"
	"sync"
	"testing"

	"github.com/dmin12/zcrypto1.3/x509"
)

const exampleCertWithOCSPDelegation = `
-----BEGIN CERTIFICATE-----
MIIFLzCCBBegAwIBAgIQQFFpI7/egSZZtXZGsGlOJDANBgkqhkiG9w0BAQsFADB+
MQswCQYDVQQGEwJVUzEdMBsGA1UEChMUU3ltYW50ZWMgQ29ycG9yYXRpb24xHzAd
BgNVBAsTFlN5bWFudGVjIFRydXN0IE5ldHdvcmsxLzAtBgNVBAMTJlN5bWFudGVj
IENsYXNzIDMgU2VjdXJlIFNlcnZlciBDQSAtIEc0MB4XDTE2MDgwMTAwMDAwMFoX
DTE4MDkzMDIzNTk1OVowgZIxCzAJBgNVBAYTAlVTMREwDwYDVQQIDAhJbGxpbm9p
czEQMA4GA1UEBwwHT2dsZXNieTEqMCgGA1UECgwhSWxsaW5vaXMgVmFsbGV5IENv
bW11bml0eSBDb2xsZWdlMRIwEAYDVQQLDAlCb29rc3RvcmUxHjAcBgNVBAMMFXd3
dy5pdmNjYm9va3N0b3JlLmNvbTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoC
ggEBAMUOWilh91JLixiaYMj9rtJPzAQh68Q/IrcmHZHH7NBeN4bBb2UwQTOpXjTw
boCdgVm1Ta4OOblk2kBLlZTHp0Zp6BYEZK3uAjmxe2NipvitFA0FkBuWJfC1Xj+S
nBjDwUqSskC92z6JnDzt3d2gZazmK69MdiuqYI2scgeCcGf2DeWvBnR+WHJ76O5d
rNcx/GvndIhqMBHd6b9yNyTsX8ZfxzCaWmIU36Z3GciWzaYV80hkBFDC4/TJ9dsS
2IW7POl8wHdzdBcHvOVYAVQKPpVRc1DQIIWQNalHHbKZ/J2SgM5G2v7ODv3eWxRM
uyzoSuBRksG+fxSUrz/QXfo9w3kCAwEAAaOCAZIwggGOMDMGA1UdEQQsMCqCFXd3
dy5pdmNjYm9va3N0b3JlLmNvbYIRaXZjY2Jvb2tzdG9yZS5jb20wCQYDVR0TBAIw
ADAOBgNVHQ8BAf8EBAMCBaAwKwYDVR0fBCQwIjAgoB6gHIYaaHR0cDovL3NzLnN5
bWNiLmNvbS9zcy5jcmwwYQYDVR0gBFowWDBWBgZngQwBAgIwTDAjBggrBgEFBQcC
ARYXaHR0cHM6Ly9kLnN5bWNiLmNvbS9jcHMwJQYIKwYBBQUHAgIwGQwXaHR0cHM6
Ly9kLnN5bWNiLmNvbS9ycGEwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMC
MB8GA1UdIwQYMBaAFF9gz2GQVd+EQxSKYCqy9Xr0QxjvMFcGCCsGAQUFBwEBBEsw
STAfBggrBgEFBQcwAYYTaHR0cDovL3NzLnN5bWNkLmNvbTAmBggrBgEFBQcwAoYa
aHR0cDovL3NzLnN5bWNiLmNvbS9zcy5jcnQwEwYKKwYBBAHWeQIEAwEB/wQCBQAw
DQYJKoZIhvcNAQELBQADggEBAJAl0wcd/QnYXtJc2PGkVMDneU29BYaSBZG4xaAU
8uWTspP+Nfb7UAcoT71oHpN8UFAiXQf1+bAorfofd1qQcZjUc5vAg04hK/r0ogI1
rLvBJe4/jW3BzFbpgNFl+I2cnY5eRz5ZL1EeKwDxpqK1gSLlTtqwkaiIynqdBCfX
lqDnqLozsE/vn2hNh3zc1zxj1Io36ALADtJOhw/HGlrabYlHh1o7XCm2/9y0scKH
rsfxMSV9FBVsbBJutTs3nfTGiMR4XISOueetlln3/2ZlNDfGXiXdy9D5/PnxbOqL
gGR2BKlwVlQR5rRkASVSMuNFHz2QN3Ddk0SQfR/aWGwiofU=
-----END CERTIFICATE-----`

const revokedCert = `
-----BEGIN CERTIFICATE-----
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
g7YS
-----END CERTIFICATE-----`

const issuerCert = `
-----BEGIN CERTIFICATE-----
MIIFODCCBCCgAwIBAgIQUT+5dDhwtzRAQY0wkwaZ/zANBgkqhkiG9w0BAQsFADCB
yjELMAkGA1UEBhMCVVMxFzAVBgNVBAoTDlZlcmlTaWduLCBJbmMuMR8wHQYDVQQL
ExZWZXJpU2lnbiBUcnVzdCBOZXR3b3JrMTowOAYDVQQLEzEoYykgMjAwNiBWZXJp
U2lnbiwgSW5jLiAtIEZvciBhdXRob3JpemVkIHVzZSBvbmx5MUUwQwYDVQQDEzxW
ZXJpU2lnbiBDbGFzcyAzIFB1YmxpYyBQcmltYXJ5IENlcnRpZmljYXRpb24gQXV0
aG9yaXR5IC0gRzUwHhcNMTMxMDMxMDAwMDAwWhcNMjMxMDMwMjM1OTU5WjB+MQsw
CQYDVQQGEwJVUzEdMBsGA1UEChMUU3ltYW50ZWMgQ29ycG9yYXRpb24xHzAdBgNV
BAsTFlN5bWFudGVjIFRydXN0IE5ldHdvcmsxLzAtBgNVBAMTJlN5bWFudGVjIENs
YXNzIDMgU2VjdXJlIFNlcnZlciBDQSAtIEc0MIIBIjANBgkqhkiG9w0BAQEFAAOC
AQ8AMIIBCgKCAQEAstgFyhx0LbUXVjnFSlIJluhL2AzxaJ+aQihiw6UwU35VEYJb
A3oNL+F5BMm0lncZgQGUWfm893qZJ4Itt4PdWid/sgN6nFMl6UgfRk/InSn4vnlW
9vf92Tpo2otLgjNBEsPIPMzWlnqEIRoiBAMnF4scaGGTDw5RgDMdtLXO637QYqzu
s3sBdO9pNevK1T2p7peYyo2qRA4lmUoVlqTObQJUHypqJuIGOmNIrLRM0XWTUP8T
L9ba4cYY9Z/JJV3zADreJk20KQnNDz0jbxZKgRb78oMQw7jW2FUyPfG9D72MUpVK
Fpd6UiFjdS8W+cRmvvW1Cdj/JwDNRHxvSz+w9wIDAQABo4IBYzCCAV8wEgYDVR0T
AQH/BAgwBgEB/wIBADAwBgNVHR8EKTAnMCWgI6Ahhh9odHRwOi8vczEuc3ltY2Iu
Y29tL3BjYTMtZzUuY3JsMA4GA1UdDwEB/wQEAwIBBjAvBggrBgEFBQcBAQQjMCEw
HwYIKwYBBQUHMAGGE2h0dHA6Ly9zMi5zeW1jYi5jb20wawYDVR0gBGQwYjBgBgpg
hkgBhvhFAQc2MFIwJgYIKwYBBQUHAgEWGmh0dHA6Ly93d3cuc3ltYXV0aC5jb20v
Y3BzMCgGCCsGAQUFBwICMBwaGmh0dHA6Ly93d3cuc3ltYXV0aC5jb20vcnBhMCkG
A1UdEQQiMCCkHjAcMRowGAYDVQQDExFTeW1hbnRlY1BLSS0xLTUzNDAdBgNVHQ4E
FgQUX2DPYZBV34RDFIpgKrL1evRDGO8wHwYDVR0jBBgwFoAUf9Nlp8Ld7LvwMAnz
Qzn6Aq8zMTMwDQYJKoZIhvcNAQELBQADggEBAF6UVkndji1l9cE2UbYD49qecxny
H1mrWH5sJgUs+oHXXCMXIiw3k/eG7IXmsKP9H+IyqEVv4dn7ua/ScKAyQmW/hP4W
Ko8/xabWo5N9Q+l0IZE1KPRj6S7t9/Vcf0uatSDpCr3gRRAMFJSaXaXjS5HoJJtG
QGX0InLNmfiIEfXzf+YzguaoxX7+0AjiJVgIcWjmzaLmFN5OUiQt/eV5E1PnXi8t
TRttQBVSK/eHiXgSgW7ZTaoteNTCLD0IX4eRnh8OsN4wUmSGiaqdZpwOdgyA8nTY
Kvi4Os7X1g8RvmurFPW9QaAiY4nxug9vKWNmLT+sjHLF+8fk1A/yO0+MKcc=
-----END CERTIFICATE-----`

const crlRevokedCert = `
-----BEGIN CERTIFICATE-----
MIIG+DCCBeCgAwIBAgIRAN9wJmL1kVzLsW66G9WlG+8wDQYJKoZIhvcNAQELBQAw
djELMAkGA1UEBhMCVVMxCzAJBgNVBAgTAk1JMRIwEAYDVQQHEwlBbm4gQXJib3Ix
EjAQBgNVBAoTCUludGVybmV0MjERMA8GA1UECxMISW5Db21tb24xHzAdBgNVBAMT
FkluQ29tbW9uIFJTQSBTZXJ2ZXIgQ0EwHhcNMTgwNDE3MDAwMDAwWhcNMjAwNDE2
MjM1OTU5WjCBqzELMAkGA1UEBhMCVVMxDjAMBgNVBBETBTAyMjE1MQswCQYDVQQI
EwJNQTEPMA0GA1UEBxMGQm9zdG9uMRcwFQYDVQQJEw5PbmUgU2lsYmVyIFdheTEm
MCQGA1UEChMdVFJVU1RFRVMgT0YgQk9TVE9OIFVOSVZFUlNJVFkxEDAOBgNVBAsT
B0luZm9TZWMxGzAZBgNVBAMTEnd3dy1mZS10ZXN0LmJ1LmVkdTCCASIwDQYJKoZI
hvcNAQEBBQADggEPADCCAQoCggEBAKWvgRHP6F/2e5aEKiKkkbxiqiij7i7Pg2zC
F4O+eyRALIydPh97JpCVhK0C8WG5uKclooI4tey/5fIKXBKug2HTXVNtVhupu9Wb
9wuFWA2xvw6PxRYlQSLaGEVnWPkJwMzbeVXiUYwHK7HRbc8dP9ivOLKHotAVaguf
iON0O//jU4mpllSH76PquYtJewZW7AgXO1K49WAYmJ6vX2D3fQTw69iSoxbdNZbN
0zx3qsQQCl4d79aSj+m0ZPzkncUVwLoNN78g030zjluiH2bo8dM2L78bPlA4mijp
sBPHal+5B18ReS51FwUCj6n7qaU7kxY81fuMvBw5jfopQ9hodesCAwEAAaOCA0kw
ggNFMB8GA1UdIwQYMBaAFB4Fo3ePbJbiW4dLprSGrHEADOc4MB0GA1UdDgQWBBS3
MLkqHskB1rVge+hV6/3yF2S/9zAOBgNVHQ8BAf8EBAMCBaAwDAYDVR0TAQH/BAIw
ADAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwZwYDVR0gBGAwXjBSBgwr
BgEEAa4jAQQDAQEwQjBABggrBgEFBQcCARY0aHR0cHM6Ly93d3cuaW5jb21tb24u
b3JnL2NlcnQvcmVwb3NpdG9yeS9jcHNfc3NsLnBkZjAIBgZngQwBAgIwRAYDVR0f
BD0wOzA5oDegNYYzaHR0cDovL2NybC5pbmNvbW1vbi1yc2Eub3JnL0luQ29tbW9u
UlNBU2VydmVyQ0EuY3JsMHUGCCsGAQUFBwEBBGkwZzA+BggrBgEFBQcwAoYyaHR0
cDovL2NydC51c2VydHJ1c3QuY29tL0luQ29tbW9uUlNBU2VydmVyQ0FfMi5jcnQw
JQYIKwYBBQUHMAGGGWh0dHA6Ly9vY3NwLnVzZXJ0cnVzdC5jb20wHQYDVR0RBBYw
FIISd3d3LWZlLXRlc3QuYnUuZWR1MIIBfwYKKwYBBAHWeQIEAgSCAW8EggFrAWkA
dgDuS723dc5guuFCaR+r4Z5mow9+X7By2IMAxHuJeqj9ywAAAWLUrlzRAAAEAwBH
MEUCIQCaaruMLPu0LBKeFsHiXVrcIu2ZPbQhJsv1uBZMdh2n9gIgFjUKIowL6qeX
5rcMmk/kdW+Zs/1niiACbXd4zCNDrHMAdgBep3P531bA57U2SH3QSeAyepGaDISh
EhKEGHWWgXFFWAAAAWLUrl5wAAAEAwBHMEUCIQDew814X7qOkBIV71mzsg7wgOlS
roQpUPKHRFluUjGvtAIgB2DhnbAg2ZzuXahjpbSoN4Mgu9BG1OGkj/1QwMDiBPcA
dwBvU3asMfAxGdiZAKRRFf93FRwR2QLBACkGjbIImjfZEwAAAWLUrly8AAAEAwBI
MEYCIQCPz2gxJlSoNaj/r0jPL7ZYg2trpzHyGpxWkVT+TRgw+AIhAO5SH33jBTeY
JPQrNm+zOEsc6HmyFtp2hbxFf+Ao9n8QMA0GCSqGSIb3DQEBCwUAA4IBAQBhV+I+
R/LVRV4dPTmUy5WRa3PGQ6KMGjQUrujrU53ubHKDQl7QmhBX79xGAsb7P4krz2Tc
O1GVV11vQMP0HQQ3IPyERuiHLOL14EygUsIZeYX+/KbuKJpErMlXZY7urxmnMSGv
k2eajyCSqEqLxnJNd0oIJHPEz9r/q7XfK9lgIV8pwgW5pqwmM4dpEvHYJLkIlXOp
7RnANs0LtBstKQa/T+zAlxsV1nUM6yu9wAJu996ToSNgc2ap+y+M7nh6o2VWPMfa
rBME9QyQdbHgGm22ukrANHwZOHB7Kv9y6RDFokBNnIHlW7/jsGXstDpUKK+DFZTN
e5qqPa6xjlp5+zXa
-----END CERTIFICATE-----`

const ocspResponseHex = `308206450a0100a082063e3082063a06092b06010505073001010482062b3082062730819ea2160414545919db6531e1afcfc6014c8e1409324c87be4a180f32303138303732373038313733345a307330713049300906052b0e03021a05000414d1b1648b8c9f0dd16ba38acd2b5017d5f9cfc06404145f60cf619055df8443148a602ab2f57af44318ef021040516923bfde812659b57646b0694e248000180f32303138303732373038313733345aa011180f32303138303830333038313733345a300d06092a864886f70d0101050500038201010012b0b7fdc3366bd2bf9db3f93bf61ac583132d2aeec4ace34fb7701bbfa1f90999a3f13c4ce91c8c8fdf5ac9ff05e67e1a86fcd7fc9a3ab446289f7cc9ed8bc54ff4ac93e62b088920b927c4dc3f58ac45c22d673ec8af15ecf42a6a876302839703b1584e08413a1aba9a9acc5a4bcee12e9d81e5e9e74539f0bada540643ec804565e89f40399bf6ec489512c4d1347b6eb591ef234cbfe3ae3cc322b14b0930d91c7126aad1a69a0892941fa5c90069a10162d1a1cd9d2b95050dbf7306c87fedae324d3b77686841007502869705899c952318a59a7cd016739c7a6c86f0bc18f88f0619ed82c1bef416d6c060726f70643aeab8fedb7c30fcffa851f957a082046e3082046a308204663082034ea003020102021010ae53bdb9affdeadcff94233538ffb8300d06092a864886f70d01010b0500307e310b3009060355040613025553311d301b060355040a131453796d616e74656320436f72706f726174696f6e311f301d060355040b131653796d616e746563205472757374204e6574776f726b312f302d0603550403132653796d616e74656320436c61737320332053656375726520536572766572204341202d204734301e170d3138303631303030303030305a170d3138303930383233353935395a3040313e303c0603550403133553796d616e74656320436c61737320332053656375726520536572766572204341202d204734204f43535020526573706f6e64657230820122300d06092a864886f70d01010105000382010f003082010a02820101008348ee9c1ee04b882ef2b5285e5f6121d121920834fdf0f90b912b56126c0152279f15209adc620c21ff511b181c170580d52cfeea24cb8eb877743fce22f2e16fa91acd12948ec36a454998cb7c70bc6916ad1ed28eff57c23f7233c8d88fb9e2da9171187a69aa8eb965e4d8f177e6c44ea507afc3f6c5c5fa730d371d4921b2428c5550c27bd3eaf44b9d099111a604d678ddfbfbb73b37570574ce6fb943332d72f6f7a47a04d703fb957c2c244f9098e29714c787abc1d83f8b2fc95d798dc59c4403021714b40e694c921dcab8b59a8cacf1ed0a3bb33f382ce3eade89736a396690f46b1e725bd558b06bcbeaeb67da11859059d3d24e2985fdf0b4b10203010001a382011c30820118300f06092b06010505073001050402050030220603551d11041b3019a4173015311330110603550403130a5447562d462d32323831301f0603551d230418301680145f60cf619055df8443148a602ab2f57af44318ef301d0603551d0e04160414545919db6531e1afcfc6014c8e1409324c87be4a300c0603551d130101ff04023000306e0603551d20046730653063060b6086480186f845010717033054302606082b06010505070201161a687474703a2f2f7777772e73796d617574682e636f6d2f637073302a06082b06010505070202301e1a1c2020687474703a2f2f7777772e73796d617574682e636f6d2f72706130130603551d25040c300a06082b06010505070309300e0603551d0f0101ff040403020780300d06092a864886f70d01010b0500038201010022d3cfb4fe8e4c760ff9cfe2ac30d9025a2a52873ec1feeb5744792388240df905fce0ef778dea6249200c78dc89d81263d125570a11c55ab8251c3977d22307f9df717d574bddc3a57aa94ec01e442e070a2247fffd77b7f309bcdcac37cd715840b3e7091308a1aeaf9bc53459fc1305dbc6cff03620e8f5f92489dd15579fb1dd18a052090886d5a99447f2e63c3283a78b3ce09b62d2f44565b68b4a4bfc67192a44256fb481b00a625c6e2559af945e6cd3f2196e35cee7b7a79736c02877f0b4a06acb7dd9da802e74fec75465facb94c260a25556bbe6b683c1674342f5687605dbdff4c4c384e263d640f5e4efd22a52e9e2ebe77b628ccb0b9deca1`

const ocspRevokedHex = `3082065d0a0100a08206563082065206092b0601050507300101048206433082063f3081b6a2160414545919db6531e1afcfc6014c8e1409324c87be4a180f32303138303733313032343031355a30818a3081873049300906052b0e03021a05000414d1b1648b8c9f0dd16ba38acd2b5017d5f9cfc06404145f60cf619055df8443148a602ab2f57af44318ef0210009e79e98f8373ac096b08ca05b5365ca116180f32303137303930383031303934395aa0030a0105180f32303138303733313032343031355aa011180f32303138303830373032343031355a300d06092a864886f70d0101050500038201010063c6159aef3b79ced6c09e32a910928537a980455a83a0a985757d04aa911d2da930f06ce9e5c7af635b140d511358f574d9b08360bb38efe3a9788b3773915d208526521cac654ea4c215e5c833fbe3e24bf47d131a05a32f509f26207fa3cecbf165555b2d2902f2fccd297ee11c88e0b867435d5c1291d1fa3a8ef6d55abb827972bafff68b2de746ceec2d75f8bfee610599d921dcc4fc6da34e6226946f2248cd75f886b2ca11eb5228217c7a1f8d106c9e97c1ec6959d20b6d249ca8f2b49fd4b2ac6d9fbf0ab6d34f99743b75c5c2fd2a6697e7fa46234992b2e0ac6874d9ce76a77eefad9a29676fc4a180ffd3321b1c90dd82e6ec30a6e9942bb5d4a082046e3082046a308204663082034ea003020102021010ae53bdb9affdeadcff94233538ffb8300d06092a864886f70d01010b0500307e310b3009060355040613025553311d301b060355040a131453796d616e74656320436f72706f726174696f6e311f301d060355040b131653796d616e746563205472757374204e6574776f726b312f302d0603550403132653796d616e74656320436c61737320332053656375726520536572766572204341202d204734301e170d3138303631303030303030305a170d3138303930383233353935395a3040313e303c0603550403133553796d616e74656320436c61737320332053656375726520536572766572204341202d204734204f43535020526573706f6e64657230820122300d06092a864886f70d01010105000382010f003082010a02820101008348ee9c1ee04b882ef2b5285e5f6121d121920834fdf0f90b912b56126c0152279f15209adc620c21ff511b181c170580d52cfeea24cb8eb877743fce22f2e16fa91acd12948ec36a454998cb7c70bc6916ad1ed28eff57c23f7233c8d88fb9e2da9171187a69aa8eb965e4d8f177e6c44ea507afc3f6c5c5fa730d371d4921b2428c5550c27bd3eaf44b9d099111a604d678ddfbfbb73b37570574ce6fb943332d72f6f7a47a04d703fb957c2c244f9098e29714c787abc1d83f8b2fc95d798dc59c4403021714b40e694c921dcab8b59a8cacf1ed0a3bb33f382ce3eade89736a396690f46b1e725bd558b06bcbeaeb67da11859059d3d24e2985fdf0b4b10203010001a382011c30820118300f06092b06010505073001050402050030220603551d11041b3019a4173015311330110603550403130a5447562d462d32323831301f0603551d230418301680145f60cf619055df8443148a602ab2f57af44318ef301d0603551d0e04160414545919db6531e1afcfc6014c8e1409324c87be4a300c0603551d130101ff04023000306e0603551d20046730653063060b6086480186f845010717033054302606082b06010505070201161a687474703a2f2f7777772e73796d617574682e636f6d2f637073302a06082b06010505070202301e1a1c2020687474703a2f2f7777772e73796d617574682e636f6d2f72706130130603551d25040c300a06082b06010505070309300e0603551d0f0101ff040403020780300d06092a864886f70d01010b0500038201010022d3cfb4fe8e4c760ff9cfe2ac30d9025a2a52873ec1feeb5744792388240df905fce0ef778dea6249200c78dc89d81263d125570a11c55ab8251c3977d22307f9df717d574bddc3a57aa94ec01e442e070a2247fffd77b7f309bcdcac37cd715840b3e7091308a1aeaf9bc53459fc1305dbc6cff03620e8f5f92489dd15579fb1dd18a052090886d5a99447f2e63c3283a78b3ce09b62d2f44565b68b4a4bfc67192a44256fb481b00a625c6e2559af945e6cd3f2196e35cee7b7a79736c02877f0b4a06acb7dd9da802e74fec75465facb94c260a25556bbe6b683c1674342f5687605dbdff4c4c384e263d640f5e4efd22a52e9e2ebe77b628ccb0b9deca1`

func startServer(issuer *x509.Certificate, wg *sync.WaitGroup) {
	goodResp, err := hex.DecodeString(ocspResponseHex)
	if err != nil {
		panic(err)
	}
	badResp, err := hex.DecodeString(ocspRevokedHex)
	if err != nil {
		panic(err)
	}
	http.HandleFunc("/issuer", func(w http.ResponseWriter, r *http.Request) {
		w.Write(issuer.Raw)
	})
	http.HandleFunc("/issuermalformed", func(w http.ResponseWriter, r *http.Request) {
		w.Write(issuer.Raw[0:30])
	})
	http.HandleFunc("/goodcertrequest", func(w http.ResponseWriter, r *http.Request) {
		w.Write(goodResp)
	})
	http.HandleFunc("/badcertrequest", func(w http.ResponseWriter, r *http.Request) {
		w.Write(badResp)
	})
	http.HandleFunc("/malformedresponse", func(w http.ResponseWriter, r *http.Request) {
		w.Write(badResp[0:30])
	})
	http.HandleFunc("/revlist", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./testdata/test.crl")
	})
	http.HandleFunc("/revlistmalformed", func(w http.ResponseWriter, r *http.Request) {
		w.Write(badResp[0:30]) // intentionally send the wrong type, and truncated
	})
	wg.Done()
	http.ListenAndServe(":8080", nil)
}

func init() {
	_, _, issuer, _ := parseCertPEM()
	var bootWG sync.WaitGroup
	bootWG.Add(1)
	go startServer(issuer, &bootWG)
	bootWG.Wait()
}

func parseCertPEM() (cert *x509.Certificate, revoked *x509.Certificate, issuer *x509.Certificate, crlRevoked *x509.Certificate) {
	certPool := x509.NewCertPool()
	ok := certPool.AppendCertsFromPEM([]byte(exampleCertWithOCSPDelegation))
	if !ok {
		panic("failed to parse testing cert")
	}
	ok = certPool.AppendCertsFromPEM([]byte(issuerCert))
	if !ok {
		panic("failed to parse testing cert")
	}
	ok = certPool.AppendCertsFromPEM([]byte(revokedCert))
	if !ok {
		panic("failed to parse testing cert")
	}
	ok = certPool.AppendCertsFromPEM([]byte(crlRevokedCert))
	if !ok {
		panic("failed to parse testing cert")
	}
	cert = certPool.Certificates()[0]
	issuer = certPool.Certificates()[1]
	revoked = certPool.Certificates()[2]
	crlRevoked = certPool.Certificates()[3]
	return
}

func TestOCSPGood(t *testing.T) {
	cert, _, issuer, _ := parseCertPEM()
	cert.OCSPServer[0] = "http://localhost:8080/goodcertrequest"
	isRevoked, _, err := CheckOCSP(context.Background(), cert, issuer)
	if err != nil {
		t.Error(err.Error())
	}
	if isRevoked != false {
		t.Fail()
	}
}

func TestOCSPBad(t *testing.T) {
	_, revoked, issuer, _ := parseCertPEM()
	revoked.OCSPServer[0] = "http://localhost:8080/badcertrequest"
	isRevoked, _, err := CheckOCSP(context.Background(), revoked, issuer)
	if err != nil {
		t.Error(err.Error())
	}
	if isRevoked != true {
		t.Fail()
	}
}

func TestOCSPGoodWithoutIssuer(t *testing.T) {
	cert, _, _, _ := parseCertPEM()
	cert.OCSPServer[0] = "http://localhost:8080/goodcertrequest"
	cert.IssuingCertificateURL[0] = "http://localhost:8080/issuer"
	isRevoked, _, err := CheckOCSP(context.Background(), cert, nil)
	if err != nil {
		t.Error(err.Error())
	}
	if isRevoked != false {
		t.Fail()
	}
}

func TestOCSPMalformedResponse(t *testing.T) {
	cert, _, _, _ := parseCertPEM()
	cert.OCSPServer[0] = "http://localhost:8080/malformedresponse"
	cert.IssuingCertificateURL[0] = "http://localhost:8080/issuer"
	_, _, err := CheckOCSP(context.Background(), cert, nil)
	if err == nil {
		t.Fail()
	}
}

func TestOCSPBadProtocol(t *testing.T) {
	cert, _, _, _ := parseCertPEM()
	cert.SerialNumber = nil
	cert.OCSPServer[0] = "http://localhost:8080/goodcertrequest"
	cert.IssuingCertificateURL[0] = "http://localhost:8080/issuer"
	_, _, err := CheckOCSP(context.Background(), cert, nil)
	if err == nil {
		t.Fail()
	}
}

func TestOCSPCannotConstruct(t *testing.T) {
	cert, _, _, _ := parseCertPEM()
	cert.OCSPServer[0] = "ftp://localhost:8080/"
	cert.IssuingCertificateURL[0] = "http://localhost:8080/issuer"
	_, _, err := CheckOCSP(context.Background(), cert, nil)
	if err == nil {
		t.Fail()
	}
}

func TestOCSPCannotConstruct2(t *testing.T) {
	cert, _, _, _ := parseCertPEM()
	cert.OCSPServer[0] = "http://localhost:8080/"
	cert.IssuingCertificateURL[0] = "ftp://localhost:8080/issuer"
	_, _, err := CheckOCSP(context.Background(), cert, nil)
	if err == nil {
		t.Fail()
	}
}

func TestOCSPBadCertResponse(t *testing.T) {
	cert, _, _, _ := parseCertPEM()
	cert.OCSPServer[0] = "http://localhost:8080/"
	cert.IssuingCertificateURL[0] = "http://localhost:8080/issuermalformed"
	_, _, err := CheckOCSP(context.Background(), cert, nil)
	if err == nil {
		t.Fail()
	}
}

func TestOCSPGoodWithoutIssuerNoIssuingParty(t *testing.T) {
	cert, _, _, _ := parseCertPEM()
	cert.OCSPServer[0] = "http://localhost:8080/goodcertrequest"
	cert.IssuingCertificateURL = nil
	_, _, err := CheckOCSP(context.Background(), cert, nil)
	if err == nil {
		t.Fail()
	}
}

func TestCRLRevoked(t *testing.T) {
	_, _, _, crlRevoked := parseCertPEM()
	crlRevoked.CRLDistributionPoints[0] = "http://localhost:8080/revlist"
	isRevoked, _, err := CheckCRL(context.Background(), crlRevoked, nil)
	if err != nil {
		t.Error(err.Error())
	}
	if isRevoked != true {
		t.Fail()
	}
}

func TestCRLMalformed(t *testing.T) {
	_, err := GetCRL(context.Background(), "http://localhost:8080/revlistmalformed")
	if err == nil {
		t.Fail()
	}
}

func TestCRLFailLDAP(t *testing.T) {
	_, err := GetCRL(context.Background(), "ldap://localhost:8080/revlist")
	if err == nil {
		t.Fail()
	}
}

func TestCRLFailFTP(t *testing.T) {
	_, err := GetCRL(context.Background(), "ftp://localhost:8080/revlist")
	if err == nil {
		t.Fail()
	}
}
