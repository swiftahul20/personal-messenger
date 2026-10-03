import { useState, type FormEvent } from "react";
import { api } from "./api";
import type { User } from "./types";
import { inputClass, primaryButton, Window } from "./Window";

interface SignInProps {
  slot: number;
  blockedUsername?: string;
  onSignedIn: (user: User) => void;
}

const USERNAME_PATTERN = /^[A-Za-z0-9_]{3,50}$/;

export function SignIn({ slot, blockedUsername, onSignedIn }: SignInProps) {
  const [username, setUsername] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    const name = username.trim();
    if (!USERNAME_PATTERN.test(name)) {
      setError("Use 3 to 50 letters, numbers, or underscores.");
      return;
    }
    if (
      blockedUsername &&
      name.toLowerCase() === blockedUsername.toLowerCase()
    ) {
      setError(
        `${blockedUsername} is already signed in in the other window. Use a different username.`,
      );
      return;
    }
    setError(null);
    setPending(true);
    try {
      onSignedIn(await api.signIn(name));
    } catch (failure) {
      setError(
        failure instanceof Error ? failure.message : "Could not sign in.",
      );
      setPending(false);
    }
  }

  const inputId = `username-${slot}`;

  return (
    <Window
      title="Sign in"
      label={`Window ${slot}, sign in`}
      className="h-full w-full"
    >
      <div className="flex min-h-0 flex-1 items-center justify-center overflow-y-auto bg-panel p-4">
        <form onSubmit={submit} className="w-full max-w-xs" noValidate>
          <p className="mb-3 text-muted">
            Pick a username. If it does not exist yet, it is created. There is
            no password.
          </p>
          <label htmlFor={inputId} className="mb-1 block font-bold">
            Username
          </label>
          <input
            id={inputId}
            className={inputClass}
            value={username}
            onChange={(event) => setUsername(event.target.value)}
            autoComplete="off"
            autoCapitalize="none"
            spellCheck={false}
            aria-describedby={error ? `${inputId}-error` : undefined}
            aria-invalid={error ? true : undefined}
          />
          {error ? (
            <p
              id={`${inputId}-error`}
              role="alert"
              className="mt-2 font-bold text-danger"
            >
              {error}
            </p>
          ) : null}
          <button
            type="submit"
            className={`${primaryButton} mt-3 w-full`}
            disabled={pending}
          >
            {pending ? "Signing in..." : "Sign in"}
          </button>
        </form>
      </div>
    </Window>
  );
}
