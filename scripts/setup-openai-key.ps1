param(
    [string]$Model = "gpt-4o-mini",
    [string]$BaseURL = "https://api.openai.com/v1"
)

$ErrorActionPreference = "Stop"

Write-Host "Relationship Agent Runtime - OpenAI API key setup"
Write-Host ""
Write-Host "Paste your API key in this local PowerShell window."
Write-Host "The key will be saved to your Windows User environment variables."
Write-Host "It will not be written into the repository."
Write-Host ""

$secureKey = Read-Host "OPENAI_API_KEY" -AsSecureString
$ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secureKey)
try {
    $plainKey = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr)
} finally {
    [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr)
}

if ([string]::IsNullOrWhiteSpace($plainKey)) {
    throw "OPENAI_API_KEY cannot be empty."
}

[Environment]::SetEnvironmentVariable("OPENAI_API_KEY", $plainKey, "User")
[Environment]::SetEnvironmentVariable("OPENAI_MODEL", $Model, "User")
[Environment]::SetEnvironmentVariable("OPENAI_BASE_URL", $BaseURL, "User")

$env:OPENAI_API_KEY = $plainKey
$env:OPENAI_MODEL = $Model
$env:OPENAI_BASE_URL = $BaseURL

Write-Host ""
Write-Host "Saved:"
Write-Host "OPENAI_API_KEY = ********"
Write-Host "OPENAI_MODEL = $Model"
Write-Host "OPENAI_BASE_URL = $BaseURL"
Write-Host ""
Write-Host "Close and reopen PowerShell, or start Runtime from this same window."
