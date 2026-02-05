export namespace main {
	
	export class TorrentStatus {
	    id: string;
	    name: string;
	    progress: number;
	    downSpeed: number;
	    upSpeed: number;
	    peers: number;
	    seeds: number;
	    size: number;
	    downloaded: number;
	    status: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new TorrentStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.progress = source["progress"];
	        this.downSpeed = source["downSpeed"];
	        this.upSpeed = source["upSpeed"];
	        this.peers = source["peers"];
	        this.seeds = source["seeds"];
	        this.size = source["size"];
	        this.downloaded = source["downloaded"];
	        this.status = source["status"];
	        this.error = source["error"];
	    }
	}

}

