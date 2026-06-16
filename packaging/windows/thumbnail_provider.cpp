// Windows Explorer thumbnail handler for Facet model files (.fct/.3mf/.obj/.stl).
//
// Implements the COM IThumbnailProvider interface. Explorer (via the thumbnail
// host process) calls GetThumbnail; this handler shells out to `facetc` to
// render the model to a temporary PNG, then decodes it with WIC into the HBITMAP
// Explorer expects. Unlike the macOS Quick Look extension, a Windows thumbnail
// handler is NOT sandboxed, so spawning facetc is allowed.
//
// Build: see build.sh (zig c++ → facet_thumbnail.dll). Register with regsvr32 /
// install.ps1. facetc.exe must sit next to the DLL or be on PATH.

#include <windows.h>
#include <shlobj.h>      // IInitializeWithFile
#include <shlwapi.h>     // QISearch, path helpers
#include <thumbcache.h>  // IThumbnailProvider
#include <wincodec.h>    // WIC PNG decode
#include <new>
#include <string>

// Stable CLSID for this handler: {B7A9C3E2-1F4D-4A8B-9C6E-2D5F8A1B3C4D}
static const CLSID CLSID_FacetThumb =
    {0xb7a9c3e2, 0x1f4d, 0x4a8b, {0x9c, 0x6e, 0x2d, 0x5f, 0x8a, 0x1b, 0x3c, 0x4d}};

// {e357fccd-a995-4576-b01f-234630154e96} — the IThumbnailProvider shellex slot.
static const wchar_t *kThumbIID =
    L"\\ShellEx\\{e357fccd-a995-4576-b01f-234630154e96}";

// Extensions that get Facet 3D thumbnails. facetc renders meshes directly, so
// all four share one handler.
static const wchar_t *kExtensions[] = {L".fct", L".3mf", L".obj", L".stl"};

static volatile long g_refs = 0;
static HINSTANCE g_inst = nullptr;

// --- helpers ---------------------------------------------------------------

// facetcPath returns the path to facetc.exe next to this DLL, or "facetc.exe"
// (resolved via PATH) when that isn't present.
static std::wstring facetcPath() {
    wchar_t dll[MAX_PATH];
    if (GetModuleFileNameW(g_inst, dll, MAX_PATH)) {
        std::wstring p(dll);
        size_t slash = p.find_last_of(L"\\/");
        if (slash != std::wstring::npos) {
            std::wstring cand = p.substr(0, slash + 1) + L"facetc.exe";
            if (GetFileAttributesW(cand.c_str()) != INVALID_FILE_ATTRIBUTES) {
                return cand;
            }
        }
    }
    return L"facetc.exe";
}

// runFacetc renders src to a temporary PNG at size px and returns its path, or
// an empty string on failure. The caller deletes the file.
static std::wstring runFacetc(const std::wstring &src, UINT px) {
    wchar_t dir[MAX_PATH], tmp[MAX_PATH];
    if (!GetTempPathW(MAX_PATH, dir) || !GetTempFileNameW(dir, L"fct", 0, tmp)) {
        return L"";
    }
    std::wstring out(tmp); // a .tmp file; -format png makes facetc ignore the ext

    std::wstring cmd = L"\"" + facetcPath() + L"\" \"" + src + L"\" -o \"" + out +
                       L"\" -format png -size " + std::to_wstring(px);

    STARTUPINFOW si = {sizeof(si)};
    si.dwFlags = STARTF_USESHOWWINDOW;
    si.wShowWindow = SW_HIDE;
    PROCESS_INFORMATION pi = {};
    std::wstring mutableCmd = cmd; // CreateProcessW may modify the buffer
    if (!CreateProcessW(nullptr, &mutableCmd[0], nullptr, nullptr, FALSE,
                        CREATE_NO_WINDOW, nullptr, nullptr, &si, &pi)) {
        DeleteFileW(out.c_str());
        return L"";
    }
    DWORD wait = WaitForSingleObject(pi.hProcess, 20000); // 20s cap
    if (wait != WAIT_OBJECT_0) {
        // Timed out or wait failed: kill the orphan so it releases the temp file.
        TerminateProcess(pi.hProcess, 1);
        WaitForSingleObject(pi.hProcess, 5000);
    }
    DWORD code = 1;
    GetExitCodeProcess(pi.hProcess, &code);
    CloseHandle(pi.hThread);
    CloseHandle(pi.hProcess);
    if (wait != WAIT_OBJECT_0 || code != 0) {
        DeleteFileW(out.c_str());
        return L"";
    }
    return out;
}

// pngToHBITMAP decodes a PNG file into a 32-bit premultiplied-BGRA HBITMAP.
static HRESULT pngToHBITMAP(const std::wstring &png, HBITMAP *out) {
    *out = nullptr;
    IWICImagingFactory *factory = nullptr;
    HRESULT hr = CoCreateInstance(CLSID_WICImagingFactory, nullptr,
                                  CLSCTX_INPROC_SERVER, IID_PPV_ARGS(&factory));
    if (FAILED(hr)) return hr;

    IWICBitmapDecoder *decoder = nullptr;
    IWICBitmapFrameDecode *frame = nullptr;
    IWICFormatConverter *conv = nullptr;
    hr = factory->CreateDecoderFromFilename(png.c_str(), nullptr, GENERIC_READ,
                                            WICDecodeMetadataCacheOnLoad, &decoder);
    if (SUCCEEDED(hr)) hr = decoder->GetFrame(0, &frame);
    if (SUCCEEDED(hr)) hr = factory->CreateFormatConverter(&conv);
    if (SUCCEEDED(hr)) {
        hr = conv->Initialize(frame, GUID_WICPixelFormat32bppPBGRA,
                              WICBitmapDitherTypeNone, nullptr, 0.0,
                              WICBitmapPaletteTypeCustom);
    }
    UINT w = 0, h = 0;
    if (SUCCEEDED(hr)) hr = conv->GetSize(&w, &h);
    if (SUCCEEDED(hr)) {
        BITMAPINFO bi = {};
        bi.bmiHeader.biSize = sizeof(BITMAPINFOHEADER);
        bi.bmiHeader.biWidth = (LONG)w;
        bi.bmiHeader.biHeight = -(LONG)h; // top-down
        bi.bmiHeader.biPlanes = 1;
        bi.bmiHeader.biBitCount = 32;
        bi.bmiHeader.biCompression = BI_RGB;
        void *bits = nullptr;
        HBITMAP bmp = CreateDIBSection(nullptr, &bi, DIB_RGB_COLORS, &bits, nullptr, 0);
        if (bmp && bits) {
            UINT stride = w * 4;
            hr = conv->CopyPixels(nullptr, stride, stride * h, (BYTE *)bits);
            if (SUCCEEDED(hr)) {
                *out = bmp;
            } else {
                DeleteObject(bmp);
            }
        } else {
            hr = E_FAIL;
        }
    }
    if (conv) conv->Release();
    if (frame) frame->Release();
    if (decoder) decoder->Release();
    factory->Release();
    return hr;
}

// --- the provider ----------------------------------------------------------

class FacetThumb : public IInitializeWithFile, public IThumbnailProvider {
    long m_ref = 1;
    std::wstring m_path;

public:
    FacetThumb() { InterlockedIncrement(&g_refs); }
    ~FacetThumb() { InterlockedDecrement(&g_refs); }

    IFACEMETHODIMP QueryInterface(REFIID riid, void **ppv) {
        static const QITAB qit[] = {
            QITABENT(FacetThumb, IInitializeWithFile),
            QITABENT(FacetThumb, IThumbnailProvider),
            {nullptr, 0},
        };
        return QISearch(this, qit, riid, ppv);
    }
    IFACEMETHODIMP_(ULONG) AddRef() { return InterlockedIncrement(&m_ref); }
    IFACEMETHODIMP_(ULONG) Release() {
        long r = InterlockedDecrement(&m_ref);
        if (r == 0) delete this;
        return r;
    }

    IFACEMETHODIMP Initialize(LPCWSTR path, DWORD) {
        m_path = path ? path : L"";
        return S_OK;
    }

    IFACEMETHODIMP GetThumbnail(UINT cx, HBITMAP *phbmp, WTS_ALPHATYPE *pAlpha) {
        if (phbmp) *phbmp = nullptr;
        if (pAlpha) *pAlpha = WTSAT_ARGB;
        if (m_path.empty() || !phbmp) return E_FAIL;

        std::wstring png = runFacetc(m_path, cx);
        if (png.empty()) return E_FAIL;
        HRESULT hr = pngToHBITMAP(png, phbmp);
        DeleteFileW(png.c_str());
        return hr;
    }
};

// --- class factory ---------------------------------------------------------

class Factory : public IClassFactory {
    long m_ref = 1;

public:
    Factory() { InterlockedIncrement(&g_refs); }
    ~Factory() { InterlockedDecrement(&g_refs); }

    IFACEMETHODIMP QueryInterface(REFIID riid, void **ppv) {
        if (riid == IID_IUnknown || riid == IID_IClassFactory) {
            *ppv = static_cast<IClassFactory *>(this);
            AddRef();
            return S_OK;
        }
        *ppv = nullptr;
        return E_NOINTERFACE;
    }
    IFACEMETHODIMP_(ULONG) AddRef() { return InterlockedIncrement(&m_ref); }
    IFACEMETHODIMP_(ULONG) Release() {
        long r = InterlockedDecrement(&m_ref);
        if (r == 0) delete this;
        return r;
    }
    IFACEMETHODIMP CreateInstance(IUnknown *outer, REFIID riid, void **ppv) {
        if (outer) return CLASS_E_NOAGGREGATION;
        FacetThumb *p = new (std::nothrow) FacetThumb();
        if (!p) return E_OUTOFMEMORY;
        HRESULT hr = p->QueryInterface(riid, ppv);
        p->Release();
        return hr;
    }
    IFACEMETHODIMP LockServer(BOOL lock) {
        InterlockedAdd(&g_refs, lock ? 1 : -1);
        return S_OK;
    }
};

// --- DLL exports -----------------------------------------------------------

STDAPI DllGetClassObject(REFCLSID clsid, REFIID riid, void **ppv) {
    if (clsid != CLSID_FacetThumb) return CLASS_E_CLASSNOTAVAILABLE;
    Factory *f = new (std::nothrow) Factory();
    if (!f) return E_OUTOFMEMORY;
    HRESULT hr = f->QueryInterface(riid, ppv);
    f->Release();
    return hr;
}

STDAPI DllCanUnloadNow() { return g_refs == 0 ? S_OK : S_FALSE; }

static LSTATUS setKey(HKEY root, const wchar_t *sub, const wchar_t *name,
                      const wchar_t *val) {
    HKEY k;
    LSTATUS s = RegCreateKeyExW(root, sub, 0, nullptr, 0, KEY_WRITE, nullptr, &k, nullptr);
    if (s != ERROR_SUCCESS) return s;
    s = RegSetValueExW(k, name, 0, REG_SZ, (const BYTE *)val,
                       (DWORD)((wcslen(val) + 1) * sizeof(wchar_t)));
    RegCloseKey(k);
    return s;
}

STDAPI DllRegisterServer() {
    wchar_t dll[MAX_PATH];
    if (!GetModuleFileNameW(g_inst, dll, MAX_PATH)) return HRESULT_FROM_WIN32(GetLastError());

    const wchar_t *clsid = L"{B7A9C3E2-1F4D-4A8B-9C6E-2D5F8A1B3C4D}";
    std::wstring inproc = std::wstring(L"CLSID\\") + clsid + L"\\InprocServer32";
    if (setKey(HKEY_CLASSES_ROOT, (std::wstring(L"CLSID\\") + clsid).c_str(), nullptr,
               L"Facet Thumbnail Provider") != ERROR_SUCCESS) return E_FAIL;
    if (setKey(HKEY_CLASSES_ROOT, inproc.c_str(), nullptr, dll) != ERROR_SUCCESS) return E_FAIL;
    if (setKey(HKEY_CLASSES_ROOT, inproc.c_str(), L"ThreadingModel", L"Apartment") != ERROR_SUCCESS) return E_FAIL;
    for (const wchar_t *ext : kExtensions) {
        std::wstring key = std::wstring(ext) + kThumbIID;
        if (setKey(HKEY_CLASSES_ROOT, key.c_str(), nullptr, clsid) != ERROR_SUCCESS) return E_FAIL;
    }

    SHChangeNotify(SHCNE_ASSOCCHANGED, SHCNF_IDLIST, nullptr, nullptr);
    return S_OK;
}

STDAPI DllUnregisterServer() {
    const wchar_t *clsid = L"{B7A9C3E2-1F4D-4A8B-9C6E-2D5F8A1B3C4D}";
    SHDeleteKeyW(HKEY_CLASSES_ROOT, (std::wstring(L"CLSID\\") + clsid).c_str());
    for (const wchar_t *ext : kExtensions) {
        std::wstring key = std::wstring(ext) + kThumbIID;
        SHDeleteKeyW(HKEY_CLASSES_ROOT, key.c_str());
    }
    SHChangeNotify(SHCNE_ASSOCCHANGED, SHCNF_IDLIST, nullptr, nullptr);
    return S_OK;
}

BOOL WINAPI DllMain(HINSTANCE inst, DWORD reason, LPVOID) {
    if (reason == DLL_PROCESS_ATTACH) {
        g_inst = inst;
        DisableThreadLibraryCalls(inst);
    }
    return TRUE;
}
