import { afterEach, describe, expect, it, vi } from "vite-plus/test";

import {
  conditionalMediationAvailable,
  createCredential,
  getCredential,
  passkeysSupported,
} from "./webauthn";

const options = { challenge: "Y2hhbGxlbmdl", rp: { name: "Go Clean Template" } };
const parsed = { challenge: new ArrayBuffer(9) };
const requestOptions = { challenge: "Y2hhbGxlbmdl", rpId: "localhost" };
const credentialJSON = { id: "AQIDBA", type: "public-key", response: {} };

function stubBrowser(
  create: (init: unknown) => Promise<unknown>,
  get: (init: unknown) => Promise<unknown> = async () => null,
) {
  const parse = vi.fn(() => parsed);
  const parseRequest = vi.fn(() => parsed);
  const createMock = vi.fn(create);
  const getMock = vi.fn(get);
  vi.stubGlobal("PublicKeyCredential", {
    parseCreationOptionsFromJSON: parse,
    parseRequestOptionsFromJSON: parseRequest,
  });
  vi.stubGlobal("navigator", { credentials: { create: createMock, get: getMock } });

  return { parse, parseRequest, create: createMock, get: getMock };
}

const notAllowed = () =>
  new DOMException("The operation either timed out or was not allowed.", "NotAllowedError");

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("passkeysSupported", () => {
  it("is false without PublicKeyCredential", () => {
    vi.stubGlobal("PublicKeyCredential", undefined);

    expect(passkeysSupported()).toBe(false);
  });

  it("is false when the browser cannot parse JSON options", () => {
    vi.stubGlobal("PublicKeyCredential", {});
    vi.stubGlobal("navigator", { credentials: { create: vi.fn() } });

    expect(passkeysSupported()).toBe(false);
  });

  it("is false when the browser cannot parse JSON request options", () => {
    vi.stubGlobal("PublicKeyCredential", { parseCreationOptionsFromJSON: vi.fn() });
    vi.stubGlobal("navigator", { credentials: { create: vi.fn(), get: vi.fn() } });

    expect(passkeysSupported()).toBe(false);
  });

  it("is true with the JSON parsers and navigator.credentials", () => {
    stubBrowser(async () => null);

    expect(passkeysSupported()).toBe(true);
  });
});

describe("createCredential", () => {
  it("parses the options, creates the credential and returns its JSON", async () => {
    const browser = stubBrowser(async () => ({ toJSON: () => credentialJSON }));

    await expect(createCredential(options)).resolves.toEqual(credentialJSON);

    expect(browser.parse).toHaveBeenCalledWith(options);
    expect(browser.create).toHaveBeenCalledWith({ publicKey: parsed });
  });

  it("returns null when the user cancels", async () => {
    stubBrowser(async () => {
      throw new DOMException(
        "The operation either timed out or was not allowed.",
        "NotAllowedError",
      );
    });

    await expect(createCredential(options)).resolves.toBeNull();
  });

  it("rethrows any other failure", async () => {
    stubBrowser(async () => {
      throw new DOMException("The authenticator was previously registered.", "InvalidStateError");
    });

    await expect(createCredential(options)).rejects.toMatchObject({ name: "InvalidStateError" });
  });

  it("fails when the browser returns no credential", async () => {
    stubBrowser(async () => null);

    await expect(createCredential(options)).rejects.toThrow();
  });
});

describe("getCredential", () => {
  it("parses the options, gets the assertion and returns its JSON", async () => {
    const browser = stubBrowser(
      async () => null,
      async () => ({ toJSON: () => credentialJSON }),
    );

    await expect(getCredential(requestOptions)).resolves.toEqual(credentialJSON);

    expect(browser.parseRequest).toHaveBeenCalledWith(requestOptions);
    expect(browser.get).toHaveBeenCalledWith({ publicKey: parsed });
  });

  it("passes conditional mediation and the abort signal through", async () => {
    const browser = stubBrowser(
      async () => null,
      async () => ({ toJSON: () => credentialJSON }),
    );
    const { signal } = new AbortController();

    await getCredential(requestOptions, { mediation: "conditional", signal });

    expect(browser.get).toHaveBeenCalledWith({
      publicKey: parsed,
      mediation: "conditional",
      signal,
    });
  });

  it("returns null when the user cancels", async () => {
    stubBrowser(
      async () => null,
      async () => {
        throw notAllowed();
      },
    );

    await expect(getCredential(requestOptions)).resolves.toBeNull();
  });

  it("returns null when the request is aborted", async () => {
    const controller = new AbortController();
    stubBrowser(
      async () => null,
      async () => {
        controller.abort();
        throw controller.signal.reason;
      },
    );

    await expect(getCredential(requestOptions, { signal: controller.signal })).resolves.toBeNull();
  });

  it("rethrows any other failure", async () => {
    stubBrowser(
      async () => null,
      async () => {
        throw new DOMException("The relying party ID is not valid.", "SecurityError");
      },
    );

    await expect(getCredential(requestOptions)).rejects.toMatchObject({ name: "SecurityError" });
  });

  it("fails when the browser returns no credential", async () => {
    stubBrowser(async () => null);

    await expect(getCredential(requestOptions)).rejects.toThrow();
  });
});

describe("conditionalMediationAvailable", () => {
  function stubConditional(available: () => Promise<boolean>) {
    stubBrowser(async () => null);
    vi.stubGlobal("PublicKeyCredential", {
      parseCreationOptionsFromJSON: vi.fn(),
      parseRequestOptionsFromJSON: vi.fn(),
      isConditionalMediationAvailable: vi.fn(available),
    });
  }

  it("is false without passkey support", async () => {
    vi.stubGlobal("PublicKeyCredential", undefined);

    await expect(conditionalMediationAvailable()).resolves.toBe(false);
  });

  it("is false when the browser has no conditional mediation check", async () => {
    stubBrowser(async () => null);

    await expect(conditionalMediationAvailable()).resolves.toBe(false);
  });

  it("is what the browser reports", async () => {
    stubConditional(async () => true);
    await expect(conditionalMediationAvailable()).resolves.toBe(true);

    stubConditional(async () => false);
    await expect(conditionalMediationAvailable()).resolves.toBe(false);
  });

  it("is false when the check fails", async () => {
    stubConditional(async () => {
      throw new Error("boom");
    });

    await expect(conditionalMediationAvailable()).resolves.toBe(false);
  });
});
