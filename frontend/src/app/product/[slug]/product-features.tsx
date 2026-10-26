import { Award, Hammer, Leaf, type LucideIcon } from "lucide-react";

type Feature = {
  title: string;
  description: string;
  icon?: LucideIcon;
};

type ProductFeaturesProps = {
  features: Feature[];
};

const defaultIcons = [Leaf, Hammer, Award];

export function ProductFeatures({ features }: ProductFeaturesProps) {
  return (
    <section className="mt-20 border-t border-border pt-16">
      <h2 className="mb-12 text-center text-3xl font-medium tracking-tight">
        Características del espacio
      </h2>
      <div className="grid gap-8 md:grid-cols-3">
        {features.map((feature, index) => {
          const Icon =
            feature.icon ?? defaultIcons[index % defaultIcons.length] ?? Leaf;
          return (
            <div
              key={feature.title}
              className="group flex flex-col items-center text-center"
            >
              <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-secondary transition-colors group-hover:bg-foreground">
                <Icon
                  className="h-6 w-6 text-muted-foreground transition-colors group-hover:text-background"
                  aria-hidden="true"
                />
              </div>
              <h3 className="mb-2 text-lg font-medium">{feature.title}</h3>
              <p className="text-sm text-muted-foreground">
                {feature.description}
              </p>
            </div>
          );
        })}
      </div>
    </section>
  );
}
