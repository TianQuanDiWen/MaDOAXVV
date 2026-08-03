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
    param(
        [string]$Path,
        [switch]$Quiet
    )

    if (-not (Test-Path -LiteralPath $Path)) {
        if (-not $Quiet) {
            Write-Host "Already clean: $Path"
        }
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

    if (-not $Quiet) {
        Write-Host "Removing: $ResolvedPath"
    }
    Remove-Item -LiteralPath $ResolvedPath -Recurse -Force
}

function Clear-LocalBuildResources {
    param([string[]]$Paths)

    Write-Step "Cleaning configured local build resources"
    foreach ($Path in $Paths) {
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
        "User-Agent" = "$ProjectName-MXU-builder"
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

function Remove-TemporaryDirectory {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
        return
    }

    for ($Attempt = 1; $Attempt -le 3; $Attempt++) {
        try {
            Remove-SafeDirectory -Path $Path -Quiet
            return
        }
        catch {
            if ($Attempt -eq 3) {
                throw
            }
            Start-Sleep -Milliseconds 300
        }
    }
}

function Install-GitHubReleaseAsset {
    param(
        [string]$Repository,
        [string]$AssetPattern,
        [string]$Release,
        [string]$Destination,
        [string]$CacheDirectory,
        [hashtable]$Headers
    )

    $ReleaseUri = if ($Release -eq "latest") {
        "https://api.github.com/repos/$Repository/releases/latest"
    }
    else {
        $EncodedRelease = [Uri]::EscapeDataString($Release)
        "https://api.github.com/repos/$Repository/releases/tags/$EncodedRelease"
    }

    Write-Step "Resolving $Repository release"
    $ReleaseInfo = Invoke-RestMethod -Uri $ReleaseUri -Headers $Headers
    $Asset = $ReleaseInfo.assets |
        Where-Object {
            $_.name -like $AssetPattern -and
            (
                $_.name -match '\.zip$' -or
                $_.name -match '\.(tar\.gz|tgz)$'
            )
        } |
        Select-Object -First 1

    if (-not $Asset) {
        throw "No release asset matched '$AssetPattern' in release '$($ReleaseInfo.tag_name)'."
    }

    New-Item -ItemType Directory -Force -Path $CacheDirectory | Out-Null
    $ArchivePath = Join-Path $CacheDirectory $Asset.name
    if (-not (Test-Path -LiteralPath $ArchivePath -PathType Leaf)) {
        Write-Step "Downloading $($Asset.name)"
        Download-File `
            -Uri $Asset.browser_download_url `
            -Destination $ArchivePath `
            -Headers $Headers
    }
    else {
        Write-Host "Using cached archive: $ArchivePath"
    }

    $ExtractName = "extract-" + [IO.Path]::GetFileNameWithoutExtension(
        $Asset.name
    )
    $ExtractDirectory = Join-Path $CacheDirectory $ExtractName
    Remove-TemporaryDirectory -Path $ExtractDirectory
    New-Item -ItemType Directory -Force -Path $ExtractDirectory | Out-Null

    Write-Step "Extracting $($Asset.name)"
    if ($Asset.name -match '\.zip$') {
        Expand-Archive `
            -LiteralPath $ArchivePath `
            -DestinationPath $ExtractDirectory `
            -Force
    }
    else {
        $Tar = Resolve-NativeCommand -Name "tar"
        Invoke-NativeCommand `
            -FilePath $Tar `
            -Arguments @("-xzf", $ArchivePath, "-C", $ExtractDirectory)
    }

    New-Item -ItemType Directory -Force -Path $Destination | Out-Null
    Get-ChildItem -LiteralPath $ExtractDirectory -Force |
        Copy-Item -Destination $Destination -Recurse -Force
    Remove-TemporaryDirectory -Path $ExtractDirectory
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
        Resolve-WorkspacePath `
            -RelativePath $ConfigPath `
            -FieldName "ConfigPath"
    }
    if (-not (Test-Path -LiteralPath $ResolvedConfigPath -PathType Leaf)) {
        throw "Build configuration was not found: $ResolvedConfigPath"
    }

    $Config = Get-Content -Raw -Encoding UTF8 $ResolvedConfigPath |
        ConvertFrom-Json
    foreach ($RequiredValue in @(
        $Config.projectName,
        $Config.defaultTarget,
        $Config.dependencies.maaFramework.repository,
        $Config.dependencies.maaFramework.release,
        $Config.dependencies.maaFramework.assetPattern,
        $Config.dependencies.mxu.repository,
        $Config.dependencies.mxu.release,
        $Config.dependencies.mxu.assetPattern,
        $Config.toolchain.goVersion,
        $Config.toolchain.pythonVersion,
        $Config.toolchain.uvIndex,
        $Config.agent.source,
        $Config.agent.output,
        $Config.directories.output,
        $Config.directories.dependencies,
        $Config.directories.downloads,
        $Config.directories.cache,
        $Config.packageName
    )) {
        if ([string]::IsNullOrWhiteSpace("$RequiredValue")) {
            throw "The build configuration contains an empty required value."
        }
    }

    $ResolvedTargetName = if ($Target) {
        $Target
    }
    else {
        $Config.defaultTarget
    }
    $TargetConfig = $Config.targets |
        Where-Object {
            "$($_.platform)-$($_.arch)" -eq $ResolvedTargetName
        } |
        Select-Object -First 1
    if (-not $TargetConfig) {
        $AvailableTargets = $Config.targets |
            ForEach-Object { "$($_.platform)-$($_.arch)" }
        throw "Unknown target '$ResolvedTargetName'. Available targets: $($AvailableTargets -join ', ')"
    }

    $OutputDir = Resolve-WorkspacePath `
        -RelativePath $Config.directories.output `
        -FieldName "directories.output"
    $DependenciesDir = Resolve-WorkspacePath `
        -RelativePath $Config.directories.dependencies `
        -FieldName "directories.dependencies"
    $DownloadsDir = Resolve-WorkspacePath `
        -RelativePath $Config.directories.downloads `
        -FieldName "directories.downloads"
    $CacheDir = Resolve-WorkspacePath `
        -RelativePath $Config.directories.cache `
        -FieldName "directories.cache"
    $MxuDir = Join-Path $DownloadsDir "MXU"
    $DepsBin = Join-Path $DependenciesDir "bin"
    $DepsAgentBinary = Join-Path $DependenciesDir "share\MaaAgentBinary"
    $ResolvedVersion = Resolve-Version -RequestedVersion $Version
    $TemplateValues = @{
        project = $Config.projectName
        platform = $TargetConfig.platform
        arch = $TargetConfig.arch
        version = $ResolvedVersion
    }
    $MaaFrameworkAssetPattern = Expand-BuildTemplate `
        -Template $Config.dependencies.maaFramework.assetPattern `
        -Values $TemplateValues
    $MxuAssetPattern = Expand-BuildTemplate `
        -Template $Config.dependencies.mxu.assetPattern `
        -Values $TemplateValues

    [PSCustomObject]@{
        Action = $ResolvedAction
        Config = $ResolvedConfigPath
        Target = $ResolvedTargetName
        Version = $ResolvedVersion
        MaaFramework = $Config.dependencies.maaFramework.release
        MXU = $Config.dependencies.mxu.release
        OutputDirectory = $OutputDir
    } | Format-List

    if ($DryRun) {
        Write-Host "Dry run completed." -ForegroundColor Green
        return
    }

    $CleanPaths = @($OutputDir, $DownloadsDir, $CacheDir)
    if ($ResolvedAction -eq "Clean") {
        Clear-LocalBuildResources -Paths $CleanPaths
        Write-Host ""
        Write-Host "Local build resources cleaned." -ForegroundColor Green
        return
    }

    Write-Step "Checking build tools"
    $Uv = Resolve-NativeCommand -Name "uv"
    $Go = Resolve-NativeCommand -Name "go"
    $Npx = $null
    if (-not $SkipChecks) {
        $Npx = Resolve-NativeCommand -Name "npx"
    }

    if ($Clean) {
        Clear-LocalBuildResources -Paths $CleanPaths
    }
    else {
        Remove-SafeDirectory -Path $OutputDir -Quiet
    }

    $Headers = Get-GitHubHeaders -ProjectName $Config.projectName
    $CommonOcrDir = Join-Path $Root "assets\MaaCommonAssets\OCR"
    if (-not (Test-Path -LiteralPath $CommonOcrDir)) {
        Write-Step "Initializing MaaCommonAssets submodule"
        $Git = Resolve-NativeCommand -Name "git"
        Invoke-NativeCommand `
            -FilePath $Git `
            -Arguments @(
                "-C", $Root,
                "submodule", "update", "--init", "--depth", "1", "--",
                "assets/MaaCommonAssets"
            )
    }

    if (-not (Test-Path -LiteralPath $CommonOcrDir)) {
        throw "MaaCommonAssets OCR resources were not found."
    }

    if (
        -not (Test-Path -LiteralPath $DepsBin) -or
        -not (Test-Path -LiteralPath $DepsAgentBinary)
    ) {
        Install-GitHubReleaseAsset `
            -Repository $Config.dependencies.maaFramework.repository `
            -AssetPattern $MaaFrameworkAssetPattern `
            -Release $Config.dependencies.maaFramework.release `
            -Destination $DependenciesDir `
            -CacheDirectory $CacheDir `
            -Headers $Headers
    }
    if (
        -not (Test-Path -LiteralPath $DepsBin) -or
        -not (Test-Path -LiteralPath $DepsAgentBinary)
    ) {
        throw "MaaFramework package is missing bin or share\MaaAgentBinary."
    }

    if (-not (Test-Path -LiteralPath $MxuDir)) {
        Install-GitHubReleaseAsset `
            -Repository $Config.dependencies.mxu.repository `
            -AssetPattern $MxuAssetPattern `
            -Release $Config.dependencies.mxu.release `
            -Destination $MxuDir `
            -CacheDirectory $CacheDir `
            -Headers $Headers
    }

    Push-Location $Root
    try {
        $env:UV_INDEX = $Config.toolchain.uvIndex

        if (-not $SkipChecks) {
            Write-Step "Running Go checks"
            Invoke-NativeCommand `
                -FilePath $Go `
                -Arguments @("test", "./...")

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

        Write-Step "Installing MXU project files"
        Invoke-NativeCommand `
            -FilePath $Uv `
            -Arguments @(
                "run",
                "--python", $Config.toolchain.pythonVersion,
                "--with-requirements", "tools/requirements.txt",
                "python",
                "tools/install.py",
                $ResolvedVersion,
                $TargetConfig.platform,
                $TargetConfig.arch,
                "--config", $ResolvedConfigPath
            )
    }
    finally {
        Pop-Location
    }

    Write-Host ""
    Write-Host "MXU build completed: $OutputDir" -ForegroundColor Green
}
catch {
    $BuildError = $_
    Write-Host ""
    Write-Host "MXU build failed" -ForegroundColor Red
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
