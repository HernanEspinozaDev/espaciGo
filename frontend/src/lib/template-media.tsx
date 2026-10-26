import Image, { type ImageProps } from "next/image";

type TemplateMediaProps = Omit<ImageProps, "src" | "alt"> & {
  src: string;
  alt: string;
};

/** Local-only image adapter retaining the upstream template's media API. */
export function TemplateMedia({ src, alt, ...props }: TemplateMediaProps) {
  return <Image src={src} alt={alt} unoptimized {...props} />;
}
