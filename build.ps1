param(
    [string]$ConfigPath = "build.config.json",
    [ValidateSet("Build", "Clean")]
    [string]$Action = "",
    [string]$Target = "",
    [string]$Version = "",
    [switch]$Clean,
    [switch]$SkipChecks,
    [switch]$NoPause,
    [switch]$DryRun
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$Root = $PSScriptRoot
$env:PYTHONUTF8 = "1"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

function Write-Step {
    param([string]$Message)

    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Resolve-WorkspacePath {
    param(
        [string]$RelativePath,
        [string]$FieldName
    )

    if ([string]::IsNullOrWhiteSpace($RelativePath)) {
        throw "$FieldName cannot be empty."
    }
    if ([IO.Path]::IsPathRooted($RelativePath)) {
        throw "$FieldName must be relative to the repository root: $RelativePath"
    }

    $ResolvedRoot = [IO.Path]::GetFullPath($Root)
    $ResolvedPath = [IO.Path]::GetFullPath((Join-Path $Root $RelativePath))
    $RootPrefix = $ResolvedRoot.TrimEnd(
        [IO.Path]::DirectorySeparatorChar,
        [IO.Path]::AltDirectorySeparatorChar
    ) + [IO.Path]::DirectorySeparatorChar

    if (-not $ResolvedPath.StartsWith(
        $RootPrefix,
        [StringComparison]::OrdinalIgnoreCase
    )) {
        throw "$FieldName escapes the repository root: $RelativePath"
    }

    return $ResolvedPath
}

function Remove-SafeDirectory {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
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

    Remove-Item -LiteralPath $ResolvedPath -Recurse -Force
}

function Resolve-NativeCommand {
    param([string]$Name)

    $Command = Get-Command $Name -All -ErrorAction SilentlyContinue |
        Where-Object { $_.CommandType -eq "Application" } |
        Select-Object -First 1
    if (-not $Command) {
        throw "Required command '$Name' was not found."
    }
    return $Command.Source
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

function Resolve-Version {
    param([string]$RequestedVersion)

    if (-not [string]::IsNullOrWhiteSpace($RequestedVersion)) {
        return $RequestedVersion
    }

    $Git = Get-Command git -ErrorAction SilentlyContinue
    if ($Git) {
        $PreviousPreference = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        try {
            $Tag = & $Git.Source -C $Root describe --tags --match "v*" 2>$null |
                Select-Object -First 1
            if ($Tag) {
                return ("$Tag").Trim()
            }

            $ShortSha = & $Git.Source -C $Root rev-parse --short HEAD 2>$null |
                Select-Object -First 1
            if ($ShortSha) {
                return "v0.0.0-dev.$(("$ShortSha").Trim())"
            }
        }
        finally {
            $ErrorActionPreference = $PreviousPreference
        }
    }

    return "v0.0.0-dev"
}

function Expand-BuildTemplate {
    param(
        [string]$Template,
        [hashtable]$Values
    )

    $Result = $Template
    foreach ($Key in $Values.Keys) {
        $Result = $Result.Replace("{$Key}", "$($Values[$Key])")
    }
    return $Result
}

function Get-GitHubHeaders {
    param([string]$ProjectName)

    $Headers = @{
        Accept = "application/vnd.github+json"
        "User-Agent" = "$ProjectName-MWU-builder"
        "X-GitHub-Api-Version" = "2022-11-28"
    }
    $Token = [Environment]::GetEnvironmentVariable("GITHUB_TOKEN")
    if ($Token) {
        $Headers.Authorization = "Bearer $Token"
    }
    return $Headers
}

function Download-File {
    param(
        [string]$Uri,
        [string]$Destination,
        [hashtable]$Headers
    )

    $Parent = Split-Path -Parent $Destination
    New-Item -ItemType Directory -Force -Path $Parent | Out-Null
    Invoke-WebRequest `
        -Uri $Uri `
        -Headers $Headers `
        -OutFile $Destination `
        -UseBasicParsing
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

function Clear-LocalBuildResources {
    param([string[]]$Paths)

    Write-Step "Cleaning configured local build resources"
    foreach ($Path in $Paths) {
        if (Test-Path -LiteralPath $Path) {
            Write-Host "Removing: $Path"
            Remove-SafeDirectory -Path $Path
        }
        else {
            Write-Host "Already clean: $Path"
        }
    }
}

function Wait-OnBuildFailure {
    if (
        $NoPause -or
        $env:CI -or
        -not [Environment]::UserInteractive -or
        [Console]::IsInputRedirected
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

$ResolvedConfigPath = if ([IO.Path]::IsPathRooted($ConfigPath)) {
    [IO.Path]::GetFullPath($ConfigPath)
}
else {
    Resolve-WorkspacePath -RelativePath $ConfigPath -FieldName "ConfigPath"
}

if (-not (Test-Path -LiteralPath $ResolvedConfigPath -PathType Leaf)) {
    throw "Build configuration was not found: $ResolvedConfigPath"
}

$Config = Get-Content -Raw -Encoding UTF8 $ResolvedConfigPath |
    ConvertFrom-Json

foreach ($RequiredValue in @(
    $Config.projectName,
    $Config.defaultTarget,
    $Config.mwu.repository,
    $Config.mwu.ref,
    $Config.mwu.release,
    $Config.mwu.copyResourcesScript,
    $Config.mwu.downloadDependenciesScript,
    $Config.mwu.assetPattern,
    $Config.toolchain.pythonVersion,
    $Config.toolchain.uvIndex,
    $Config.toolchain.localExtraction.command,
    $Config.toolchain.archiveCommand,
    $Config.directories.build,
    $Config.directories.cache
)) {
    if ([string]::IsNullOrWhiteSpace("$RequiredValue")) {
        throw "The build configuration contains an empty required value."
    }
}

if (-not $Config.toolchain.localExtraction.arguments.Count) {
    throw "toolchain.localExtraction.arguments must contain at least one argument."
}

if ($Config.directories.build -ne "build") {
    throw "directories.build must be 'build' because MWU official scripts write there."
}

$ResolvedTargetName = if ($Target) { $Target } else { $Config.defaultTarget }
$TargetConfig = $Config.targets |
    Where-Object { "$($_.platform)-$($_.arch)" -eq $ResolvedTargetName } |
    Select-Object -First 1
if (-not $TargetConfig) {
    $AvailableTargets = $Config.targets |
        ForEach-Object { "$($_.platform)-$($_.arch)" }
    throw "Unknown target '$ResolvedTargetName'. Available targets: $($AvailableTargets -join ', ')"
}

$ResolvedVersion = Resolve-Version -RequestedVersion $Version
$TemplateValues = @{
    project = $Config.projectName
    platform = $TargetConfig.platform
    arch = $TargetConfig.arch
    version = $ResolvedVersion
}
$AssetPattern = Expand-BuildTemplate `
    -Template $Config.mwu.assetPattern `
    -Values $TemplateValues

$BuildDir = Resolve-WorkspacePath `
    -RelativePath $Config.directories.build `
    -FieldName "directories.build"
$CacheDir = Resolve-WorkspacePath `
    -RelativePath $Config.directories.cache `
    -FieldName "directories.cache"

$BuildSummary = [PSCustomObject]@{
    Action = $ResolvedAction
    Config = $ResolvedConfigPath
    Target = $ResolvedTargetName
    Version = $ResolvedVersion
    MwuRepository = $Config.mwu.repository
    MwuRef = $Config.mwu.ref
    MwuRelease = $Config.mwu.release
    AssetPattern = $AssetPattern
    BuildDirectory = $BuildDir
    Extractor = $Config.toolchain.localExtraction.command
}
$BuildSummary | Format-List

if ($DryRun) {
    Write-Host "Dry run completed." -ForegroundColor Green
    return
}

if ($ResolvedAction -eq "Clean") {
    Clear-LocalBuildResources -Paths @($BuildDir, $CacheDir)
    Write-Host ""
    Write-Host "Local build resources cleaned." -ForegroundColor Green
    return
}

Write-Step "Checking build tools"
$Uv = Resolve-NativeCommand -Name "uv"
$Extractor = Resolve-NativeCommand `
    -Name $Config.toolchain.localExtraction.command
$Npx = $null
if (-not $SkipChecks) {
    $Npx = Resolve-NativeCommand -Name "npx"
}

if ($Clean) {
    Clear-LocalBuildResources -Paths @($BuildDir, $CacheDir)
}
else {
    Remove-SafeDirectory -Path $BuildDir
}

New-Item -ItemType Directory -Force -Path $CacheDir | Out-Null
$ScriptsDir = Join-Path $CacheDir "mwu-scripts"
New-Item -ItemType Directory -Force -Path $ScriptsDir | Out-Null

$Headers = Get-GitHubHeaders -ProjectName $Config.projectName
$RawBase = "https://raw.githubusercontent.com/$($Config.mwu.repository)/$($Config.mwu.ref)"
$CopyScript = Join-Path $ScriptsDir "copy_resources.py"
$DependencyScript = Join-Path $ScriptsDir "download_deps.py"

Write-Step "Downloading MWU official build scripts"
Download-File `
    -Uri "$RawBase/$($Config.mwu.copyResourcesScript)" `
    -Destination $CopyScript `
    -Headers $Headers
Download-File `
    -Uri "$RawBase/$($Config.mwu.downloadDependenciesScript)" `
    -Destination $DependencyScript `
    -Headers $Headers

Push-Location $Root
try {
    $env:UV_INDEX = $Config.toolchain.uvIndex
    $env:tag = $ResolvedVersion

    if (-not $SkipChecks) {
        Write-Step "Running resource checks"
        Invoke-NativeCommand `
            -FilePath $Npx `
            -Arguments @("@nekosu/maa-tools", "check")
        Invoke-NativeCommand `
            -FilePath $Uv `
            -Arguments @(
                "run",
                "--python", $Config.toolchain.pythonVersion,
                "--with", "jsonschema==4.26.0",
                "--with", "referencing==0.37.0",
                "python",
                "tools/validate_schema.py",
                "--schema-dir", "deps/tools",
                "--resource-dirs", "assets/resource",
                "--exclude-dirs", "assets/resource/announcement",
                "--interface-files", "assets/interface.json"
            )
    }

    Write-Step "Copying project resources with MWU official script"
    Invoke-NativeCommand `
        -FilePath $Uv `
        -Arguments @(
            "run",
            "--python", $Config.toolchain.pythonVersion,
            $CopyScript
        )

    Write-Step "Preparing Agent dependencies with MWU official script"
    Invoke-NativeCommand `
        -FilePath $Uv `
        -Arguments @(
            "run",
            "--python", $Config.toolchain.pythonVersion,
            $DependencyScript
        )
}
finally {
    Pop-Location
}

$ReleaseUri = if ($Config.mwu.release -eq "latest") {
    "https://api.github.com/repos/$($Config.mwu.repository)/releases/latest"
}
else {
    $EncodedRelease = [Uri]::EscapeDataString($Config.mwu.release)
    "https://api.github.com/repos/$($Config.mwu.repository)/releases/tags/$EncodedRelease"
}

Write-Step "Resolving MWU release asset"
$Release = Invoke-RestMethod -Uri $ReleaseUri -Headers $Headers
$Asset = $Release.assets |
    Where-Object { $_.name -like $AssetPattern } |
    Select-Object -First 1
if (-not $Asset) {
    throw "No MWU release asset matched '$AssetPattern' in release '$($Release.tag_name)'."
}

$ArchivePath = Join-Path $CacheDir $Asset.name
if (-not (Test-Path -LiteralPath $ArchivePath -PathType Leaf)) {
    Write-Step "Downloading $($Asset.name)"
    Download-File `
        -Uri $Asset.browser_download_url `
        -Destination $ArchivePath `
        -Headers $Headers
}
else {
    Write-Host "Using cached MWU archive: $ArchivePath"
}

Write-Step "Extracting MWU runtime"
New-Item -ItemType Directory -Force -Path $BuildDir | Out-Null
$ExtractionValues = @{
    archive = $ArchivePath
    destination = $BuildDir
}
$ExtractionArguments = @(
    $Config.toolchain.localExtraction.arguments |
        ForEach-Object {
            Expand-BuildTemplate `
                -Template "$_" `
                -Values $ExtractionValues
        }
)
Invoke-NativeCommand `
    -FilePath $Extractor `
    -Arguments $ExtractionArguments

Write-Host ""
Write-Host "MWU build completed: $BuildDir" -ForegroundColor Green
}
catch {
    $BuildError = $_
    Write-Host ""
    Write-Host "MWU build failed" -ForegroundColor Red
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
