export namespace evaluator {
	
	export class DebugMesh {
	    role: string;
	    mesh?: manifold.DisplayMesh;
	
	    static createFrom(source: any = {}) {
	        return new DebugMesh(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.mesh = this.convertValues(source["mesh"], manifold.DisplayMesh);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace main {
	
	export class AssistantConfig {
	    cli: string;
	    model: string;
	    systemPrompt: string;
	    maxTurns: number;
	
	    static createFrom(source: any = {}) {
	        return new AssistantConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cli = source["cli"];
	        this.model = source["model"];
	        this.systemPrompt = source["systemPrompt"];
	        this.maxTurns = source["maxTurns"];
	    }
	}
	export class CLIInfo {
	    id: string;
	    name: string;
	    models: string[];
	    defaultModel: string;
	
	    static createFrom(source: any = {}) {
	        return new CLIInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.models = source["models"];
	        this.defaultModel = source["defaultModel"];
	    }
	}
	export class DocGuide {
	    title: string;
	    slug: string;
	    markdown: string;
	
	    static createFrom(source: any = {}) {
	        return new DocGuide(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.slug = source["slug"];
	        this.markdown = source["markdown"];
	    }
	}
	export class LibraryInfo {
	    id: string;
	    name: string;
	    ref: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new LibraryInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.ref = source["ref"];
	        this.path = source["path"];
	    }
	}
	export class SlicerInfo {
	    name: string;
	    id: string;
	
	    static createFrom(source: any = {}) {
	        return new SlicerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.id = source["id"];
	    }
	}

}

export namespace manifold {
	
	export class DisplayMesh {
	    VertexCount: number;
	    IndexCount: number;
	    FaceGroupCount: number;
	
	    static createFrom(source: any = {}) {
	        return new DisplayMesh(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.VertexCount = source["VertexCount"];
	        this.IndexCount = source["IndexCount"];
	        this.FaceGroupCount = source["FaceGroupCount"];
	    }
	}

}

