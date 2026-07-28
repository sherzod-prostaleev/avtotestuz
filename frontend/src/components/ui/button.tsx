import * as React from "react";

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "game" | "success" | "gold" | "outline" | "ghost" | "destructive";
  size?: "sm" | "md" | "lg";
  /**
   * Use `span` when the control is nested inside a Link/anchor.
   * Nested <button> inside <a> is invalid HTML and clicks often do nothing.
   */
  as?: "button" | "span";
}

const variantStyles: Record<string, string> = {
  game: "bg-accent text-accent-foreground shadow-3d hover:brightness-105 active:translate-y-1 active:shadow-none border-b-4 border-accent-shadow",
  success:
    "bg-success text-success-foreground shadow-3d-success hover:brightness-105 active:translate-y-1 active:shadow-none border-b-4 border-success-shadow",
  gold: "bg-gold text-slate-950 shadow-3d-gold hover:brightness-105 active:translate-y-1 active:shadow-none border-b-4 border-gold-shadow font-extrabold",
  outline: "border border-border bg-card text-foreground hover:border-accent hover:text-foreground",
  ghost: "text-muted-foreground hover:bg-card hover:text-foreground",
  destructive:
    "bg-danger text-danger-foreground border-b-4 border-danger-shadow hover:brightness-105 active:translate-y-1 active:shadow-none",
};

const sizeStyles: Record<string, string> = {
  sm: "min-h-11 h-11 px-4 text-xs rounded-xl",
  md: "min-h-11 h-11 px-5 text-sm rounded-xl",
  lg: "min-h-12 h-12 px-7 text-base rounded-2xl",
};

function buttonClassName(
  variant: NonNullable<ButtonProps["variant"]>,
  size: NonNullable<ButtonProps["size"]>,
  className: string,
  opts?: { disabledVisual?: boolean; disabledPseudo?: boolean }
) {
  const disabledPart = opts?.disabledPseudo
    ? "disabled:pointer-events-none disabled:opacity-50"
    : opts?.disabledVisual
      ? "pointer-events-none opacity-50"
      : "";
  return `inline-flex items-center justify-center font-bold tracking-wide transition-all duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${disabledPart} ${variantStyles[variant]} ${sizeStyles[size]} ${className}`;
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      className = "",
      variant = "game",
      size = "md",
      children,
      disabled,
      as = "button",
      type = "button",
      ...props
    },
    ref
  ) => {
    // as="span" renders a button-looking element with no button semantics,
    // for use inside a <Link> (an <a><button> nesting is invalid HTML).
    // The span carries the visible label, so it must stay in the
    // accessibility tree: it is what gives the wrapping anchor its
    // accessible name. Marking it aria-hidden left every such link
    // announced as a bare "link" with no name.
    if (as === "span") {
      return (
        <span
          className={buttonClassName(variant, size, className, {
            disabledVisual: Boolean(disabled),
          })}
        >
          {children}
        </span>
      );
    }

    return (
      <button
        ref={ref}
        type={type}
        disabled={disabled}
        className={buttonClassName(variant, size, className, { disabledPseudo: true })}
        {...props}
      >
        {children}
      </button>
    );
  }
);
Button.displayName = "Button";
