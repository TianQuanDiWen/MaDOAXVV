param(
    [ValidateSet("Build", "Clean")]
    [string]$Action = "",
    [string]$Version = "",
    [string]$PythonPath = "",
    [ValidateSet("win", "linux", "macos", "android")]
    [string]$TargetOS = "win",
    [ValidateSet("x86_64", "aarch64")]
    [string]$Arch = "x86_64",
    [string]$MaaFrameworkVersion = "",
    [string]$MfaAvaloniaVersion = "",
    [switch]$Clean,
    [switch]$SkipNodeCheck,
    [switch]$SkipPythonDeps,
    [switch]$SkipSchemaCheck,
    [switch]$NoPause,
    [switch]$Package
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$Root = $PSScriptRoot
$InstallDir = Join-Path $Root "install"
$DistDir = Join-Path $Root "dist"
$DepsBin = Join-Path $Root "deps\bin"
$DepsAgentBinary = Join-Path $Root "deps\share\MaaAgentBinary"
$MfaDir = Join-Path $Root "MFA"
$DownloadCacheDir = Join-Path $Root "cache\downloads"
$env:PYTHONUTF8 = "1"

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Remove-SafeDirectory {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
        Write-Host "Already clean: $Path"
        return
    }

    $ResolvedRoot = [IO.Path]::GetFullPath($Root)
    $ResolvedPath = (Resolve-Path -LiteralPath $Path).Path
    $RootPrefix = $ResolvedRoot.TrimEnd(
        [IO.Path]::DirectorySeparatorChar,
        [IO.Path]::AltDirectorySeparatorChar
    ) + [IO.Path]::DirectorySeparatorChar

    if (-not $ResolvedPath.StartsWith(
        $RootPrefix,
        [StringComparison]::OrdinalIgnoreCase
    )) {
        throw "Refusing to remove a directory outside the repository: $ResolvedPath"
    }

    Write-Host "Removing: $ResolvedPath"
    Remove-Item -LiteralPath $ResolvedPath -Recurse -Force
}

function Clear-LocalBuildResources {
    Write-Step "Cleaning local build resources"
    foreach ($Path in @($InstallDir, $DistDir, $DownloadCacheDir)) {
        Remove-SafeDirectory -Path $Path
    }
}

function Test-InteractiveConsole {
    try {
        return (
            [Environment]::UserInteractive -and
            -not [Console]::IsInputRedirected
        )
    }
    catch {
        return $false
    }
}

function Select-LocalAction {
    $Options = @(
        [PSCustomObject]@{
            Number = "1"
            Value = "Build"
            Label = "Build"
        },
        [PSCustomObject]@{
            Number = "2"
            Value = "Clean"
            Label = "Clean local build resources"
        }
    )
    $SelectedIndex = 0

    Write-Host ""
    Write-Host "Select an action (arrow keys and Enter, or press 1/2):"
    $MenuTop = [Console]::CursorTop

    while ($true) {
        [Console]::SetCursorPosition(0, $MenuTop)
        for ($Index = 0; $Index -lt $Options.Count; $Index++) {
            $Prefix = if ($Index -eq $SelectedIndex) { ">" } else { " " }
            $Line = "$Prefix $($Options[$Index].Number). $($Options[$Index].Label)"
            if ($Index -eq $SelectedIndex) {
                Write-Host $Line.PadRight(40) -ForegroundColor Green
            }
            else {
                Write-Host $Line.PadRight(40)
            }
        }

        $PressedKey = [Console]::ReadKey($true).Key
        switch ($PressedKey) {
            { $_ -in @("UpArrow", "LeftArrow") } {
                $SelectedIndex = (
                    $SelectedIndex - 1 + $Options.Count
                ) % $Options.Count
            }
            { $_ -in @("DownArrow", "RightArrow") } {
                $SelectedIndex = (
                    $SelectedIndex + 1
                ) % $Options.Count
            }
            { $_ -in @("D1", "NumPad1") } {
                Write-Host ""
                return "Build"
            }
            { $_ -in @("D2", "NumPad2") } {
                Write-Host ""
                return "Clean"
            }
            "Enter" {
                Write-Host ""
                return $Options[$SelectedIndex].Value
            }
        }
    }
}

function Wait-OnBuildFailure {
    if (
        $NoPause -or
        $env:CI -or
        -not (Test-InteractiveConsole)
    ) {
        return
    }

    Write-Host ""
    Write-Host "Press any key to close..." -ForegroundColor Yellow
    try {
        [void][Console]::ReadKey($true)
    }
    catch {
        [void](Read-Host "Unable to read a single key; press Enter to close")
    }
}

function Require-Command {
    param([string]$Command)
    if (-not (Get-Command $Command -ErrorAction SilentlyContinue)) {
        throw "Required command '$Command' was not found in PATH."
    }
}

function Resolve-Python {
    param([string]$RequestedPath)

    $Candidates = [System.Collections.Generic.List[string]]::new()

    if ($RequestedPath) {
        $Candidates.Add($RequestedPath)
    }
    elseif ($env:PYTHON) {
        $Candidates.Add($env:PYTHON)
    }

    foreach ($CommandName in @("python", "python3")) {
        Get-Command $CommandName -All -ErrorAction SilentlyContinue |
            Where-Object { $_.CommandType -eq "Application" } |
            ForEach-Object { $Candidates.Add($_.Source) }
    }

    foreach ($Candidate in ($Candidates | Select-Object -Unique)) {
        try {
            $VersionOutput = & $Candidate --version 2>&1
            if ($LASTEXITCODE -eq 0 -and "$VersionOutput" -match '^Python 3\.') {
                if (Test-Path -LiteralPath $Candidate) {
                    return (Resolve-Path -LiteralPath $Candidate).Path
                }

                return $Candidate
            }
        }
        catch {
            continue
        }
    }

    if ($RequestedPath) {
        throw "The requested Python executable '$RequestedPath' is unavailable or is not Python 3."
    }

    throw "Python 3 was not found. Pass -PythonPath <path> or set the PYTHON environment variable."
}

function Invoke-NativeCommand {
    param(
        [string]$FilePath,
        [string[]]$Arguments
    )

    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Command '$FilePath' failed with exit code $LASTEXITCODE."
    }
}

function Remove-TemporaryDirectory {
    param(
        [string]$Path,
        [switch]$BestEffort
    )

    if (-not (Test-Path $Path)) {
        return
    }

    for ($Attempt = 1; $Attempt -le 3; $Attempt++) {
        try {
            Remove-Item -LiteralPath $Path -Recurse -Force -ErrorAction Stop
            return
        }
        catch {
            if ($Attempt -lt 3) {
                Start-Sleep -Milliseconds 300
                continue
            }

            if ($BestEffort) {
                Write-Warning "Could not clean temporary directory '$Path': $($_.Exception.Message)"
                return
            }

            throw
        }
    }
}

function Install-GitHubReleaseAsset {
    param(
        [string]$Repository,
        [string]$AssetPattern,
        [string]$Tag,
        [string]$Destination
    )

    $Headers = @{
        Accept = "application/vnd.github+json"
        "User-Agent" = "MaDOAXVV-local-build"
        "X-GitHub-Api-Version" = "2022-11-28"
    }
    $GitHubToken = [Environment]::GetEnvironmentVariable("GITHUB_TOKEN")
    if ($GitHubToken) {
        $Headers.Authorization = "Bearer $GitHubToken"
    }

    if ($Tag) {
        $EncodedTag = [Uri]::EscapeDataString($Tag)
        $ReleaseUri = "https://api.github.com/repos/$Repository/releases/tags/$EncodedTag"
    }
    else {
        $ReleaseUri = "https://api.github.com/repos/$Repository/releases/latest"
    }

    Write-Step "Resolving $Repository release"
    $Release = Invoke-RestMethod -Uri $ReleaseUri -Headers $Headers
    $Asset = $Release.assets |
        Where-Object {
            $_.name -like $AssetPattern -and
            ($_.name -match '\.zip$' -or $_.name -match '\.tar\.gz$')
        } |
        Select-Object -First 1

    if (-not $Asset) {
        throw "No release asset matching '$AssetPattern' was found in $Repository release '$($Release.tag_name)'."
    }

    New-Item -ItemType Directory -Force -Path $DownloadCacheDir | Out-Null
    $ArchivePath = Join-Path $DownloadCacheDir $Asset.name
    if (-not (Test-Path $ArchivePath)) {
        Write-Step "Downloading $($Asset.name)"
        Invoke-WebRequest -Uri $Asset.browser_download_url -Headers $Headers -OutFile $ArchivePath -UseBasicParsing
    }
    else {
        Write-Host "Using cached archive: $ArchivePath"
    }

    $ExtractDir = Join-Path $DownloadCacheDir ("extract-" + [IO.Path]::GetFileNameWithoutExtension($Asset.name))
    Remove-TemporaryDirectory -Path $ExtractDir
    New-Item -ItemType Directory -Force -Path $ExtractDir | Out-Null

    Write-Step "Extracting $($Asset.name)"
    if ($Asset.name -match '\.zip$') {
        Expand-Archive -LiteralPath $ArchivePath -DestinationPath $ExtractDir -Force
    }
    else {
        Require-Command tar
        Invoke-NativeCommand "tar" @("-xzf", $ArchivePath, "-C", $ExtractDir)
    }

    New-Item -ItemType Directory -Force -Path $Destination | Out-Null
    Get-ChildItem -LiteralPath $ExtractDir -Force |
        Copy-Item -Destination $Destination -Recurse -Force
    Remove-TemporaryDirectory -Path $ExtractDir -BestEffort
}

function Install-MissingBuildDependencies {
    $CommonOcrDir = Join-Path $Root "assets\MaaCommonAssets\OCR"
    if (-not (Test-Path $CommonOcrDir)) {
        Write-Step "Initializing MaaCommonAssets submodule"
        Require-Command git
        Invoke-NativeCommand "git" @(
            "-C", $Root,
            "submodule", "update", "--init", "--depth", "1", "--",
            "assets/MaaCommonAssets"
        )
    }

    if (-not (Test-Path $CommonOcrDir)) {
        throw "MaaCommonAssets OCR resources were not found after initializing the submodule."
    }

    if (-not (Test-Path $DepsBin) -or -not (Test-Path $DepsAgentBinary)) {
        Install-GitHubReleaseAsset `
            -Repository "MaaXYZ/MaaFramework" `
            -AssetPattern "MAA-$TargetOS-$Arch*" `
            -Tag $MaaFrameworkVersion `
            -Destination (Join-Path $Root "deps")
    }

    if (-not (Test-Path $DepsBin) -or -not (Test-Path $DepsAgentBinary)) {
        throw "Downloaded MaaFramework package did not contain the required bin and share\MaaAgentBinary directories."
    }

    if ($TargetOS -eq "android" -or (Test-Path $MfaDir)) {
        return
    }

    $MfaOS = switch ($TargetOS) {
        "win" { "win" }
        "macos" { "osx" }
        "linux" { "linux" }
    }
    $MfaArch = if ($Arch -eq "x86_64") { "x64" } else { "arm64" }

    Install-GitHubReleaseAsset `
        -Repository "SweetSmellFox/MFAAvalonia" `
        -AssetPattern "MFAAvalonia-*-$MfaOS-$MfaArch*" `
        -Tag $MfaAvaloniaVersion `
        -Destination $MfaDir

    if (-not (Test-Path $MfaDir)) {
        throw "MFAAvalonia download did not create the MFA directory."
    }
}

function Resolve-Version {
    if ($Version) {
        return $Version
    }

    function Invoke-GitText {
        param([string[]]$Arguments)

        if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
            return ""
        }

        $previousErrorActionPreference = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        try {
            $output = & git -C $Root @Arguments 2>$null
            if ($LASTEXITCODE -eq 0 -and $output) {
                return ($output | Select-Object -First 1).Trim()
            }
        }
        finally {
            $ErrorActionPreference = $previousErrorActionPreference
        }

        return ""
    }

    $tag = Invoke-GitText @("describe", "--tags", "--match", "v*")
    if ($tag) {
        return $tag
    }

    $shortSha = Invoke-GitText @("rev-parse", "--short", "HEAD")
    if ($shortSha) {
        return "v0.0.0-dev.$shortSha"
    }

    return "v0.0.0-dev"
}

try {
    $ResolvedAction = if ($Action) {
        $Action
    }
    elseif ($PSBoundParameters.Count -eq 0 -and (Test-InteractiveConsole)) {
        Select-LocalAction
    }
    else {
        "Build"
    }

    if ($ResolvedAction -eq "Clean") {
        Clear-LocalBuildResources
        Write-Host ""
        Write-Host "Local build resources cleaned." -ForegroundColor Green
        return
    }

    Push-Location $Root
    try {
        $ResolvedVersion = Resolve-Version

        Write-Step "Checking local tools"
        $Python = Resolve-Python -RequestedPath $PythonPath
        Write-Host "Using Python: $Python"

        if (-not $SkipNodeCheck) {
            Require-Command npm
            Require-Command npx
        }

        Install-MissingBuildDependencies

        if ($Clean) {
            Clear-LocalBuildResources
        }

        if (-not $SkipPythonDeps) {
            Write-Step "Installing Python build dependencies"
            Invoke-NativeCommand $Python @("-m", "pip", "install", "-r", "tools\requirements.txt")
            Invoke-NativeCommand $Python @("-m", "pip", "install", "jsonschema==4.26.0", "referencing==0.37.0")
        }

        if (-not $SkipNodeCheck) {
            Write-Step "Running maa-tools checks"
            if (-not (Test-Path (Join-Path $Root "node_modules"))) {
                Invoke-NativeCommand "npm" @("ci")
            }
            Invoke-NativeCommand "npx" @("@nekosu/maa-tools", "check")
        }

        if (-not $SkipSchemaCheck) {
            Write-Step "Validating Maa resource schemas"
            Invoke-NativeCommand $Python @(
                "tools\validate_schema.py",
                "--schema-dir", "deps\tools",
                "--resource-dirs", "assets\resource",
                "--interface-files", "assets\interface.json"
            )
        }

        Write-Step "Installing project files"
        New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

        Invoke-NativeCommand $Python @("tools\install.py", $ResolvedVersion, $TargetOS, $Arch)

        if ($Package) {
            Write-Step "Creating zip package"
            New-Item -ItemType Directory -Force -Path $DistDir | Out-Null
            $PackageName = "MaDOAXVV-$ResolvedVersion-$TargetOS-$Arch.zip"
            $PackagePath = Join-Path $DistDir $PackageName
            if (Test-Path $PackagePath) {
                Remove-Item -LiteralPath $PackagePath -Force
            }
            Compress-Archive -Path (Join-Path $InstallDir "*") -DestinationPath $PackagePath
            Write-Host "Package: $PackagePath" -ForegroundColor Green
        }

        Write-Host ""
        Write-Host "Build completed: $InstallDir" -ForegroundColor Green
    }
    finally {
        Pop-Location
    }
}
catch {
    $BuildError = $_
    Write-Host ""
    Write-Host "Local build failed" -ForegroundColor Red
    Write-Host $BuildError.Exception.Message -ForegroundColor Red

    if ($BuildError.InvocationInfo.PositionMessage) {
        Write-Host $BuildError.InvocationInfo.PositionMessage -ForegroundColor DarkRed
    }
    if ($BuildError.ScriptStackTrace) {
        Write-Host "Stack trace:" -ForegroundColor DarkRed
        Write-Host $BuildError.ScriptStackTrace -ForegroundColor DarkRed
    }

    Wait-OnBuildFailure
    throw
}
