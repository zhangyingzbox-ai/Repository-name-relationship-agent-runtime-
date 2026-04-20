param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$UserID = "review-demo"
)

$ErrorActionPreference = "Stop"
$OutputEncoding = [System.Text.UTF8Encoding]::new()
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new()

function Invoke-Chat([string]$message) {
    $body = @{
        user_id = $UserID
        message = $message
    } | ConvertTo-Json

    Invoke-RestMethod -Uri "$BaseUrl/chat" -Method POST -ContentType "application/json; charset=utf-8" -Body $body |
        ConvertTo-Json -Depth 10
}

function From-Utf8Base64([string]$value) {
    [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($value))
}

Write-Host "Round 1: build profile memory"
Invoke-Chat (From-Utf8Base64 "5oiR5Y+r5p6X5aSP77yM5oiR5Zyo5LiK5rW377yM5piv5Lqn5ZOB57uP55CG44CC5oiR5Zac5qyi5aSc6LeR77yM5LiN5Zac5qyi5aSq5Ya35Yaw5Yaw55qE5Zue5aSN44CC")

Write-Host ""
Write-Host "Round 2: use relationship memory"
Invoke-Chat (From-Utf8Base64 "5pyA6L+R5LiL5ZGo6KaB6Z2i6K+V77yM5oiR5pyJ54K554Sm6JmR77yM5biM5pyb5L2g5rip5p+U5LiA54K577yM5L2G5Lmf57uZ5oiR55u05o6l55qE5bu66K6u44CC")

Write-Host ""
Write-Host "Round 3: update conflicting memory"
Invoke-Chat (From-Utf8Base64 "5YW25a6e5oiR5bey57uP5pCs5Yiw5rex5Zyz5LqG77yM5pyA6L+R5L2c5oGv5LiA6Iis5Lya54as5aSc44CC")

Write-Host ""
Write-Host "Persisted profile"
Invoke-RestMethod -Uri "$BaseUrl/profile/$UserID" -Method GET | ConvertTo-Json -Depth 10
